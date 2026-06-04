package verify

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/git"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	appacceptance "github.com/nilstate/scafld/v2/internal/app/acceptance"
	appverify "github.com/nilstate/scafld/v2/internal/app/verify"
	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/trust"
)

// Options configures the verify CLI adapter.
type Options struct {
	Root        string
	ReceiptPath string
	TrustedKeys string
	Target      string
	JSON        bool
	CI          bool
}

// Handler returns a CLI-compatible verify handler.
func Handler() func(context.Context, []string, io.Writer, io.Writer) int {
	return func(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
		opts, err := Parse(args)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		out, err := Run(ctx, opts)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		if !out.Passed {
			fmt.Fprintf(stderr, "verify failed: %s\n", out.Reason)
			return 3
		}
		fmt.Fprintln(stdout, "verify passed")
		return 0
	}
}

// Run loads the receipt and trusted keys, composes ports, and verifies.
func Run(ctx context.Context, opts Options) (appverify.Result, error) {
	if opts.Root == "" {
		opts.Root = "."
	}
	cfg, err := configadapter.LoadBase(ctx, opts.Root)
	if err != nil {
		return appverify.Result{}, fmt.Errorf("load config: %w", err)
	}
	target := strings.TrimSpace(opts.Target)
	ci := opts.CI || truthy(os.Getenv("CI"))
	if ci && target == "" {
		return appverify.Result{Passed: false, Reason: "missing target in CI policy"}, nil
	}
	trustedKeysPath := firstNonEmpty(opts.TrustedKeys, os.Getenv("SCAFLD_TRUSTED_KEYS"))
	if ci && trustedKeysPath == "" {
		return appverify.Result{Passed: false, Reason: "missing CI trusted keys path"}, nil
	}
	if trustedKeysPath == "" {
		trustedKeysPath = cfg.Verify.TrustedKeysPath
	}
	trusted, err := loadTrustedKeys(resolveRootPath(opts.Root, trustedKeysPath))
	if err != nil {
		return appverify.Result{}, err
	}
	receiptPath := firstNonEmpty(opts.ReceiptPath, os.Getenv("SCAFLD_RECEIPT_PATH"), cfg.Verify.ReceiptPath)
	if receiptPath == "" {
		return appverify.Result{}, errors.New("receipt path is required")
	}
	envelope, err := loadReceipt(resolveRootPath(opts.Root, receiptPath))
	if err != nil {
		return appverify.Result{}, err
	}
	execCfg := configadapter.EffectiveExecution(opts.Root, cfg.Execution)
	runner := process.Runner{DiagnosticsDir: filepath.Join(opts.Root, ".scafld", "runs", "verify", "diagnostics")}
	return appverify.Run(ctx, envelope, trusted, appverify.Policy{
		TargetCommit:    target,
		CI:              ci,
		MinIndependence: cfg.Verify.MinIndependence,
	}, appverify.Ports{
		Snapshotter:       gitSnapshotter{adapter: git.Adapter{Root: opts.Root}},
		AcceptanceRunner:  acceptanceRunner{runner: runner, root: opts.Root, env: execCfg.ProcessEnv(), timeout: execCfg.AbsoluteTimeout(), idleTimeout: execCfg.IdleTimeout()},
		AncestryChecker:   git.Adapter{Root: opts.Root},
		SignatureVerifier: signatureVerifier{},
	})
}

type gitSnapshotter struct{ adapter git.Adapter }

func (g gitSnapshotter) Snapshot(ctx context.Context, input appverify.SnapshotInput) (appverify.Snapshot, error) {
	snapshot, err := g.adapter.Snapshot(ctx, git.SnapshotInput{Scope: input.Scope, BaseRef: input.BaseRef})
	if err != nil {
		return appverify.Snapshot{}, err
	}
	digests := make(map[string]string, len(snapshot.FileDigests))
	for _, item := range snapshot.FileDigests {
		digests[item.Path] = item.SHA256
	}
	ignored := make([]string, 0, len(snapshot.IgnoredUnreviewed))
	for _, item := range snapshot.IgnoredUnreviewed {
		ignored = append(ignored, item.Path)
	}
	return appverify.Snapshot{TreeSHA: snapshot.TreeSHA, BaseCommit: snapshot.BaseCommit, FileDigests: digests, Ignored: ignored}, nil
}

type acceptanceRunner struct {
	runner      appacceptance.Runner
	root        string
	env         []string
	timeout     time.Duration
	idleTimeout time.Duration
}

func (a acceptanceRunner) RunAcceptance(ctx context.Context, criteria []receipt.Acceptance) ([]appverify.AcceptanceResult, error) {
	out := make([]appverify.AcceptanceResult, 0, len(criteria))
	for _, criterion := range criteria {
		evaluated := appacceptance.Evaluate(ctx, a.runner, appacceptance.EvaluateInput{
			Criteria:    []appacceptance.Criterion{{ID: criterion.ID, Command: criterion.Command, ExpectedKind: criterion.ExpectedKind}},
			WorkDir:     a.root,
			Env:         a.env,
			Timeout:     a.timeout,
			IdleTimeout: a.idleTimeout,
		})
		if len(evaluated.Results) == 0 {
			continue
		}
		result := evaluated.Results[0]
		out = append(out, appverify.AcceptanceResult{ID: result.ID, Status: result.Status, ExitCode: result.ExitCode})
	}
	return out, nil
}

type signatureVerifier struct{}

func (signatureVerifier) Verify(envelope receipt.Envelope, trusted trust.TrustedKeys) error {
	key, err := trusted.ActiveKey(envelope.Signature.KeyID)
	if err != nil {
		return err
	}
	pub, err := key.PublicKeyBytes()
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(envelope.Signature.Sig)
	if err != nil {
		return err
	}
	canonical, err := receipt.CanonicalBody(envelope.Body)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), canonical, sig) {
		return errors.New("invalid ed25519 signature")
	}
	return nil
}

func loadReceipt(path string) (receipt.Envelope, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return receipt.Envelope{}, fmt.Errorf("read receipt: %w", err)
	}
	return receipt.DecodeEnvelope(data)
}

func loadTrustedKeys(path string) (trust.TrustedKeys, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return trust.TrustedKeys{}, fmt.Errorf("read trusted keys: %w", err)
	}
	return trust.ParseTrustedKeys(data)
}

func resolveRootPath(root string, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(root, filepath.FromSlash(path))
}

// Parse parses verify command arguments.
func Parse(args []string) (Options, error) {
	var opts Options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.JSON = true
		case arg == "--ci":
			opts.CI = true
		case arg == "--target":
			if i+1 >= len(args) {
				return Options{}, errors.New("--target requires a value")
			}
			opts.Target = args[i+1]
			i++
		case strings.HasPrefix(arg, "--target="):
			opts.Target = strings.TrimPrefix(arg, "--target=")
		case arg == "--root":
			if i+1 >= len(args) {
				return Options{}, errors.New("--root requires a value")
			}
			opts.Root = args[i+1]
			i++
		case strings.HasPrefix(arg, "--root="):
			opts.Root = strings.TrimPrefix(arg, "--root=")
		case arg == "--trusted-keys":
			if i+1 >= len(args) {
				return Options{}, errors.New("--trusted-keys requires a value")
			}
			opts.TrustedKeys = args[i+1]
			i++
		case strings.HasPrefix(arg, "--trusted-keys="):
			opts.TrustedKeys = strings.TrimPrefix(arg, "--trusted-keys=")
		case strings.HasPrefix(arg, "-"):
			return Options{}, fmt.Errorf("unknown verify argument %q", arg)
		default:
			if opts.ReceiptPath != "" {
				return Options{}, errors.New("verify accepts one receipt path")
			}
			opts.ReceiptPath = arg
		}
	}
	return opts, nil
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
