package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
)

// ErrProviderFailed wraps provider transport and execution failures.
var ErrProviderFailed = errors.New("provider failed")

// Runner is the process execution port required by external providers.
type Runner interface {
	Run(context.Context, execution.Request) (execution.Result, error)
}

// SubmitTool describes the provider-side structured submission channel.
type SubmitTool struct {
	Name        string
	Title       string
	Description string
	Command     string
}

// AgentRequest is the protocol-neutral prompt request used by review and harden.
type AgentRequest struct {
	TaskID           string
	Prompt           string
	SchemaName       string
	SchemaJSON       string
	StrictSchemaJSON string
	SubmitTool       SubmitTool
}

// AgentResponse is the raw structured payload produced by an agent provider.
type AgentResponse struct {
	Text               string
	Provider           string
	Model              string
	SessionID          string
	OutputFormat       string
	BinarySHA256       string
	EndpointHost       string
	EvidenceProvenance []reviewevidence.Provenance
	SandboxPolicy      SandboxPolicy
	EventSummary       map[string]int
	Result             execution.Result
	RunErr             error
}

// Agent is the shared provider transport used by protocol-specific adapters.
type Agent interface {
	InvokeAgent(context.Context, AgentRequest) (AgentResponse, error)
}

// Selection contains provider choice, model, timeout, and runner configuration.
type Selection struct {
	Provider                  string
	Command                   string
	Binary                    string
	Model                     string
	CodexModel                string
	CodexModelReasoningEffort string
	ClaudeModel               string
	ClaudeEffort              string
	GeminiModel               string
	CodexBinary               string
	ClaudeBinary              string
	GeminiBinary              string
	CodexEndpointURL          string
	ClaudeEndpointURL         string
	GeminiEndpointURL         string
	CodexEndpointHost         string
	ClaudeEndpointHost        string
	GeminiEndpointHost        string
	CWD                       string
	Runner                    Runner
	Timeout                   time.Duration
	Idle                      time.Duration
	FallbackPolicy            string
	HostAgent                 string
	CommandExists             func(string) bool
}

// AutoProvider describes the concrete provider selected for provider:auto.
type AutoProvider struct {
	Provider string
	Model    string
}

// Independence stamps how separate the selected reviewer is from the host.
type Independence struct {
	Level    string `json:"level"`
	Distinct bool   `json:"distinct"`
}

// GateReviewerSelection is the provider adapter contract consumed by the gate
// path before receipt-grade sandboxing and invocation.
type GateReviewerSelection struct {
	Provider     string
	Binary       string
	Model        string
	Independence Independence
}

// RuntimeFacts carries receipt-grade provider facts that are stamped into the
// later signed gate receipt.
type RuntimeFacts struct {
	BinarySHA256       string
	EndpointHost       string
	EvidenceProvenance []reviewevidence.Provenance
	SandboxPolicy      SandboxPolicy
}

const (
	// HostAgentCodex identifies Codex as the agent currently driving scafld.
	HostAgentCodex = "codex"
	// HostAgentClaude identifies Claude as the agent currently driving scafld.
	HostAgentClaude = "claude"
	// HostAgentGemini identifies Gemini as the agent currently driving scafld.
	HostAgentGemini = "gemini"
	// IndependenceIsolationOnly is the always-available same-context-isolated floor.
	IndependenceIsolationOnly = "isolation_only"
	// IndependenceCrossVendor is the stronger known-different-vendor classification.
	IndependenceCrossVendor = "cross_vendor"
)

// Select returns the configured review provider implementation.
func Select(opts Selection) (interface {
	Invoke(context.Context, review.Request) (review.Dossier, error)
}, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Minute
	}
	if opts.Idle == 0 {
		opts.Idle = 2 * time.Minute
	}
	if opts.Command != "" {
		return CommandProvider{Command: opts.Command, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	}
	switch opts.Provider {
	case "", "auto":
		return selectAuto(opts)
	case "local":
		return LocalProvider{}, nil
	case "command":
		return nil, errors.New("--provider=command requires --provider-command")
	case "claude":
		return ClaudeProvider{Binary: first(opts.Binary, opts.ClaudeBinary), Model: first(opts.Model, opts.ClaudeModel), Effort: opts.ClaudeEffort, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	case "codex":
		return CodexProvider{Binary: first(opts.Binary, opts.CodexBinary), Model: first(opts.Model, opts.CodexModel), ModelReasoningEffort: opts.CodexModelReasoningEffort, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	case "gemini":
		return GeminiProvider{Binary: first(opts.Binary, opts.GeminiBinary), Model: first(opts.Model, opts.GeminiModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	default:
		return nil, fmt.Errorf("unknown review provider %q", opts.Provider)
	}
}

// SelectAgent returns the configured protocol-neutral provider implementation.
func SelectAgent(opts Selection) (Agent, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Minute
	}
	if opts.Idle == 0 {
		opts.Idle = 2 * time.Minute
	}
	if opts.Command != "" {
		return CommandProvider{Command: opts.Command, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	}
	switch opts.Provider {
	case "", "auto":
		return selectAutoAgent(opts)
	case "local":
		return LocalProvider{}, nil
	case "command":
		return nil, errors.New("--provider=command requires --provider-command")
	case "claude":
		return ClaudeProvider{Binary: first(opts.Binary, opts.ClaudeBinary), Model: first(opts.Model, opts.ClaudeModel), Effort: opts.ClaudeEffort, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	case "codex":
		return CodexProvider{Binary: first(opts.Binary, opts.CodexBinary), Model: first(opts.Model, opts.CodexModel), ModelReasoningEffort: opts.CodexModelReasoningEffort, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	case "gemini":
		return GeminiProvider{Binary: first(opts.Binary, opts.GeminiBinary), Model: first(opts.Model, opts.GeminiModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q", opts.Provider)
	}
}

func selectAuto(opts Selection) (interface {
	Invoke(context.Context, review.Request) (review.Dossier, error)
}, error) {
	selected, err := AutoProviderInfo(opts)
	if err != nil {
		return nil, err
	}
	return reviewProviderFor(selected.Provider, opts), nil
}

func selectAutoAgent(opts Selection) (Agent, error) {
	selected, err := AutoProviderInfo(opts)
	if err != nil {
		return nil, err
	}
	return agentProviderFor(selected.Provider, opts), nil
}

// SelectGateReviewer chooses a runnable reviewer for the receipt gate. Unlike
// AutoProviderInfo, it does not stall when only the host vendor is available:
// isolation_only still catches context contamination, self-congratulation,
// drift, forgotten acceptance criteria, and claimed-but-not-done work, while it
// forfeits protection against correlated blind spots and same-model-wrong-twice.
func SelectGateReviewer(opts Selection) (GateReviewerSelection, error) {
	hostAgent := normalizeHostAgent(opts.HostAgent)
	requested := strings.ToLower(strings.TrimSpace(opts.Provider))
	if requested != "" && requested != "auto" {
		provider := normalizeReviewerProvider(requested)
		if provider == "" {
			return GateReviewerSelection{}, fmt.Errorf("unknown receipt-grade reviewer provider %q", opts.Provider)
		}
		if !autoProviderAvailable(provider, opts) {
			return GateReviewerSelection{}, fmt.Errorf("configured receipt-grade reviewer provider %q is unavailable", provider)
		}
		return GateReviewerSelection{
			Provider:     provider,
			Binary:       providerBinary(provider, opts),
			Model:        autoProviderModel(provider, opts),
			Independence: classifyIndependence(hostAgent, provider),
		}, nil
	}
	var firstAvailable GateReviewerSelection
	for _, provider := range autoProviderOrder(opts.HostAgent) {
		if !autoProviderAvailable(provider, opts) {
			continue
		}
		selection := GateReviewerSelection{
			Provider: provider,
			Binary:   providerBinary(provider, opts),
			Model:    autoProviderModel(provider, opts),
		}
		selection.Independence = classifyIndependence(hostAgent, provider)
		if firstAvailable.Provider == "" {
			firstAvailable = selection
		}
		if selection.Independence.Level == IndependenceCrossVendor {
			return selection, nil
		}
	}
	if firstAvailable.Provider != "" {
		return firstAvailable, nil
	}
	return GateReviewerSelection{}, errors.New("no external provider found; install codex, claude, or gemini for gate review")
}

// ReceiptGradeReviewInput is the gate-only request shape for an isolated,
// receipt-stamped provider run.
type ReceiptGradeReviewInput struct {
	Selection   Selection
	HostEnviron []string
	Evidence    []reviewevidence.EvidenceFile
	Request     AgentRequest
}

// ReceiptGradeReviewResult returns provider output plus the facts later signed
// by host-gate.
type ReceiptGradeReviewResult struct {
	Response     AgentResponse
	RuntimeFacts RuntimeFacts
}

// SelectReceiptGradeAgent returns a provider configured for exact-env,
// content-hash-pinned gate review. Ordinary review/harden paths do not call it.
func SelectReceiptGradeAgent(opts Selection, hostEnviron []string) (Agent, RuntimeFacts, error) {
	return SelectReceiptGradeAgentWithEvidence(opts, hostEnviron, nil)
}

// SelectReceiptGradeAgentWithEvidence additionally materializes the evidence
// sandbox and points the reviewer at that scratch root.
func SelectReceiptGradeAgentWithEvidence(opts Selection, hostEnviron []string, evidence []reviewevidence.EvidenceFile) (Agent, RuntimeFacts, error) {
	selected, err := selectReceiptGradeProvider(opts)
	if err != nil {
		return nil, RuntimeFacts{}, err
	}
	binary, err := ResolveReceiptGradeBinary(selected.Binary)
	if err != nil {
		return nil, RuntimeFacts{}, err
	}
	// Always materialize the isolated sandbox, even when there are zero byte-bearing
	// evidence files (a deletion-only scope, or one whose only paths are withheld
	// governed files). Otherwise the reviewer would run from the live repo root with
	// host HOME/config still reachable, defeating the receipt-grade isolation the
	// gate certifies.
	sandbox, err := BuildEvidenceSandbox(evidence)
	if err != nil {
		return nil, RuntimeFacts{}, err
	}
	opts.CWD = sandbox.CWD
	endpointURL, endpointHost := providerEndpoint(selected.Provider, opts)
	extraEnv, err := receiptGradeExtraAuthEnv(selected.Provider, hostEnviron, sandbox.Home)
	if err != nil {
		if sandbox.Cleanup != nil {
			sandbox.Cleanup()
		}
		return nil, RuntimeFacts{}, err
	}
	env, err := ScrubProviderEnv(ProviderEnvInput{
		Provider:     selected.Provider,
		HostEnviron:  hostEnviron,
		ExtraEnv:     extraEnv,
		EndpointURL:  endpointURL,
		EndpointHost: endpointHost,
		RequireAuth:  true,
		MemoryHome:   sandbox.Home,
	})
	if err != nil {
		if sandbox.Cleanup != nil {
			sandbox.Cleanup()
		}
		return nil, RuntimeFacts{}, err
	}
	facts := RuntimeFacts{
		BinarySHA256:       binary.SHA256,
		EndpointHost:       env.EndpointHost,
		EvidenceProvenance: sandbox.Provenance,
		SandboxPolicy:      sandbox.Policy,
	}
	// Record honestly whether the selected provider's CLI hard-confines reads to the
	// evidence read roots, so the receipt never implies a read jail Codex does not
	// actually enforce.
	facts.SandboxPolicy.ReadRootsEnforced = providerEnforcesReadRoots(selected.Provider)
	agent := receiptGradeAgentProviderFor(selected.Provider, opts, binary, env, facts, sandbox.ArgsPolicy)
	if sandbox.Cleanup != nil {
		agent = receiptGradeSandboxAgent{Agent: agent, sandbox: sandbox, facts: facts}
	}
	return agent, facts, nil
}

// providerEnforcesReadRoots reports whether the provider CLI hard-confines the
// reviewer's reads to the evidence read roots. Claude (--add-dir) and Gemini
// (includeDirectories) do; Codex's read-only sandbox prevents writes and sets the
// working directory but does not jail reads.
func providerEnforcesReadRoots(provider string) bool {
	switch normalizeReviewerProvider(provider) {
	case "claude", "gemini":
		return true
	default:
		return false
	}
}

func receiptGradeExtraAuthEnv(provider string, hostEnviron []string, sandboxHome string) ([]string, error) {
	if normalizeReviewerProvider(provider) != "codex" {
		return nil, nil
	}
	hostCodexHome := strings.TrimSpace(hostEnvValue(hostEnviron, "CODEX_HOME"))
	// Key-based auth with no host Codex home: nothing to copy and nothing to leak.
	if hostCodexHome == "" && hostEnvHasKey(hostEnviron, "OPENAI_API_KEY") {
		return nil, nil
	}
	// Source auth.json from the explicit host CODEX_HOME if set, else ~/.codex.
	source := ""
	if hostCodexHome != "" {
		source = filepath.Join(hostCodexHome, "auth.json")
	} else if hostHome := strings.TrimSpace(hostEnvValue(hostEnviron, "HOME")); hostHome != "" {
		source = filepath.Join(hostHome, ".codex", "auth.json")
	}
	// When there is no host CODEX_HOME to neutralize and no auth.json to copy,
	// leave the env untouched rather than minting an empty sandbox Codex home.
	if hostCodexHome == "" && source == "" {
		return nil, nil
	}
	var auth []byte
	if source != "" {
		data, err := os.ReadFile(source)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read codex auth: %w", err)
		}
		if errors.Is(err, os.ErrNotExist) && hostCodexHome == "" {
			return nil, nil
		}
		auth = data
	}
	codexHome := filepath.Join(sandboxHome, ".codex")
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		return nil, fmt.Errorf("create sandbox codex auth home: %w", err)
	}
	if len(auth) > 0 {
		// Copy ONLY auth.json; the host Codex memory, sessions, and config never
		// enter the sandbox.
		if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), auth, 0o600); err != nil {
			return nil, fmt.Errorf("write sandbox codex auth: %w", err)
		}
	}
	// Always override CODEX_HOME with the clean sandbox home (it sorts after the
	// host entry in ScrubProviderEnv's map merge), so the reviewer never reads the
	// host Codex home even when the host sets CODEX_HOME directly.
	return []string{"CODEX_HOME=" + codexHome}, nil
}

// ReceiptGradeAuthAvailable reports whether the host exposes credentials that
// can survive the receipt-grade environment scrub. It checks presence only;
// provider invocation remains the authority on whether credentials are valid.
func ReceiptGradeAuthAvailable(provider string, hostEnviron []string) bool {
	switch normalizeReviewerProvider(provider) {
	case HostAgentCodex:
		if hostEnvHasKey(hostEnviron, "OPENAI_API_KEY") {
			return true
		}
		source := ""
		if home := strings.TrimSpace(hostEnvValue(hostEnviron, "CODEX_HOME")); home != "" {
			source = filepath.Join(home, "auth.json")
		} else if home := strings.TrimSpace(hostEnvValue(hostEnviron, "HOME")); home != "" {
			source = filepath.Join(home, ".codex", "auth.json")
		}
		info, err := os.Stat(source)
		return err == nil && !info.IsDir() && info.Size() > 0
	case HostAgentClaude:
		return hostEnvHasKey(hostEnviron, "ANTHROPIC_API_KEY")
	case HostAgentGemini:
		return hostEnvHasKey(hostEnviron, "GEMINI_API_KEY") || hostEnvHasKey(hostEnviron, "GOOGLE_API_KEY")
	default:
		return false
	}
}

func hostEnvHasKey(env []string, key string) bool {
	return strings.TrimSpace(hostEnvValue(env, key)) != ""
}

func hostEnvValue(env []string, key string) string {
	want := strings.ToUpper(key)
	for _, entry := range env {
		k, v, ok := strings.Cut(entry, "=")
		if ok && strings.ToUpper(strings.TrimSpace(k)) == want {
			return v
		}
	}
	return ""
}

// InvokeReceiptGradeReview runs the gate-only provider path and returns stamped
// runtime facts alongside the raw response.
func InvokeReceiptGradeReview(ctx context.Context, input ReceiptGradeReviewInput) (ReceiptGradeReviewResult, error) {
	agent, facts, err := SelectReceiptGradeAgentWithEvidence(input.Selection, input.HostEnviron, input.Evidence)
	if err != nil {
		return ReceiptGradeReviewResult{}, err
	}
	resp, err := agent.InvokeAgent(ctx, input.Request)
	resp = stampRuntimeFacts(resp, facts)
	return ReceiptGradeReviewResult{Response: resp, RuntimeFacts: facts}, err
}

// stampRuntimeFacts fills receipt-grade runtime facts onto a provider response
// when the provider did not surface them itself. It is the single owner of that
// merge, shared by the direct receipt-grade path and the sandbox-cleanup wrapper.
func stampRuntimeFacts(resp AgentResponse, facts RuntimeFacts) AgentResponse {
	if resp.BinarySHA256 == "" {
		resp.BinarySHA256 = facts.BinarySHA256
	}
	if resp.EndpointHost == "" {
		resp.EndpointHost = facts.EndpointHost
	}
	if len(resp.EvidenceProvenance) == 0 {
		resp.EvidenceProvenance = facts.EvidenceProvenance
	}
	if len(resp.SandboxPolicy.ReadRoots) == 0 {
		resp.SandboxPolicy = facts.SandboxPolicy
	}
	return resp
}

// InvokeReceiptGradeDossier runs the gate-only sandboxed reviewer and returns a
// parsed dossier plus the runtime facts stamped into the receipt. The reviewer
// reads only the evidence sandbox; env, binary, memory home, and read roots are
// pinned by SelectReceiptGradeAgentWithEvidence, and the sandbox is cleaned up
// by the wrapped agent.
func InvokeReceiptGradeDossier(ctx context.Context, input ReceiptGradeReviewInput, req review.Request) (review.Dossier, RuntimeFacts, error) {
	agent, facts, err := SelectReceiptGradeAgentWithEvidence(input.Selection, input.HostEnviron, input.Evidence)
	if err != nil {
		return review.Dossier{}, RuntimeFacts{}, err
	}
	dossier, err := invokeReviewAgent(ctx, agent, req, review.ParseTextCalibrated)
	if err != nil {
		return review.Dossier{}, facts, err
	}
	return dossier, facts, nil
}

// AutoProviderInfo returns the concrete external provider selected by auto.
//
// When scafld can tell which agent is currently driving the task, auto prefers
// the other installed provider for independent review/hardening. With the
// default disabled fallback policy, auto refuses to use the host provider as
// its own challenger.
func AutoProviderInfo(opts Selection) (AutoProvider, error) {
	hostAgent := normalizeHostAgent(opts.HostAgent)
	order := autoProviderOrder(opts.HostAgent)
	hostProviderAvailable := false
	for _, provider := range order {
		if opts.FallbackPolicy == "disable" && hostAgent != "" && provider == hostAgent {
			hostProviderAvailable = autoProviderAvailable(provider, opts)
			continue
		}
		if autoProviderAvailable(provider, opts) {
			return AutoProvider{Provider: provider, Model: autoProviderModel(provider, opts)}, nil
		}
	}
	if opts.FallbackPolicy == "disable" && hostAgent != "" && hostProviderAvailable {
		return AutoProvider{}, errors.New("no independent auto provider found; fallback_policy is disable and only the host provider is available")
	}
	return AutoProvider{}, errors.New("no external provider found; install codex, claude, or gemini, use --provider command --provider-command <cmd>, or use --provider local for development smoke tests")
}

func reviewProviderFor(provider string, opts Selection) interface {
	Invoke(context.Context, review.Request) (review.Dossier, error)
} {
	switch provider {
	case "claude":
		return ClaudeProvider{Binary: first(opts.Binary, opts.ClaudeBinary), Model: first(opts.Model, opts.ClaudeModel), Effort: opts.ClaudeEffort, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	case "gemini":
		return GeminiProvider{Binary: first(opts.Binary, opts.GeminiBinary), Model: first(opts.Model, opts.GeminiModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	default:
		return CodexProvider{Binary: first(opts.Binary, opts.CodexBinary), Model: first(opts.Model, opts.CodexModel), ModelReasoningEffort: opts.CodexModelReasoningEffort, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	}
}

func agentProviderFor(provider string, opts Selection) Agent {
	switch provider {
	case "claude":
		return ClaudeProvider{Binary: first(opts.Binary, opts.ClaudeBinary), Model: first(opts.Model, opts.ClaudeModel), Effort: opts.ClaudeEffort, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	case "gemini":
		return GeminiProvider{Binary: first(opts.Binary, opts.GeminiBinary), Model: first(opts.Model, opts.GeminiModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	default:
		return CodexProvider{Binary: first(opts.Binary, opts.CodexBinary), Model: first(opts.Model, opts.CodexModel), ModelReasoningEffort: opts.CodexModelReasoningEffort, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	}
}

func receiptGradeAgentProviderFor(provider string, opts Selection, binary ReceiptGradeBinary, env ProviderEnvResult, facts RuntimeFacts, argsPolicy SandboxArgsPolicy) Agent {
	readRoots := append([]string(nil), argsPolicy.ReadRoots...)
	switch provider {
	case "claude":
		return ClaudeProvider{Binary: binary.Path, Model: first(opts.Model, opts.ClaudeModel), Effort: opts.ClaudeEffort, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle, Env: env.Env, EnvMode: execution.EnvModeExact, BinarySHA256: facts.BinarySHA256, EndpointHost: facts.EndpointHost, ReadRoots: readRoots, MemoryAutoloadDisabled: argsPolicy.MemoryAutoloadDisabled, SandboxPolicy: facts.SandboxPolicy}
	case "gemini":
		return GeminiProvider{Binary: binary.Path, Model: first(opts.Model, opts.GeminiModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle, Env: env.Env, EnvMode: execution.EnvModeExact, BinarySHA256: facts.BinarySHA256, EndpointHost: facts.EndpointHost, ReadRoots: readRoots, MemoryAutoloadDisabled: argsPolicy.MemoryAutoloadDisabled, SandboxPolicy: facts.SandboxPolicy}
	default:
		return CodexProvider{Binary: binary.Path, Model: first(opts.Model, opts.CodexModel), ModelReasoningEffort: opts.CodexModelReasoningEffort, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle, Env: env.Env, EnvMode: execution.EnvModeExact, BinarySHA256: facts.BinarySHA256, EndpointHost: facts.EndpointHost, ReadRoots: readRoots, MemoryAutoloadDisabled: argsPolicy.MemoryAutoloadDisabled, SandboxPolicy: facts.SandboxPolicy}
	}
}

func selectReceiptGradeProvider(opts Selection) (GateReviewerSelection, error) {
	raw := strings.ToLower(strings.TrimSpace(opts.Provider))
	if raw == "" || raw == "auto" {
		return SelectGateReviewer(opts)
	}
	provider := normalizeReviewerProvider(raw)
	if provider == "" {
		return GateReviewerSelection{}, fmt.Errorf("provider %q is not receipt-grade capable", opts.Provider)
	}
	return GateReviewerSelection{Provider: provider, Binary: providerBinary(provider, opts), Model: autoProviderModel(provider, opts), Independence: classifyIndependence(opts.HostAgent, provider)}, nil
}

func autoProviderOrder(hostAgent string) []string {
	switch normalizeHostAgent(hostAgent) {
	case HostAgentCodex:
		return []string{"claude", "gemini", "codex"}
	case HostAgentClaude:
		return []string{"codex", "gemini", "claude"}
	default:
		return []string{"codex", "claude", "gemini"}
	}
}

func autoProviderAvailable(provider string, opts Selection) bool {
	switch provider {
	case "codex":
		return strings.TrimSpace(opts.Binary) != "" || strings.TrimSpace(opts.CodexBinary) != "" || commandExists(opts, "codex")
	case "claude":
		return strings.TrimSpace(opts.Binary) != "" || strings.TrimSpace(opts.ClaudeBinary) != "" || commandExists(opts, "claude")
	case "gemini":
		return strings.TrimSpace(opts.Binary) != "" || strings.TrimSpace(opts.GeminiBinary) != "" || commandExists(opts, "gemini")
	default:
		return false
	}
}

func autoProviderModel(provider string, opts Selection) string {
	switch provider {
	case "claude":
		return first(opts.Model, opts.ClaudeModel)
	case "gemini":
		return first(opts.Model, opts.GeminiModel)
	default:
		return first(opts.Model, opts.CodexModel)
	}
}

func providerBinary(provider string, opts Selection) string {
	switch provider {
	case "claude":
		return first(opts.Binary, opts.ClaudeBinary)
	case "gemini":
		return first(opts.Binary, opts.GeminiBinary)
	default:
		return first(opts.Binary, opts.CodexBinary)
	}
}

func providerEndpoint(provider string, opts Selection) (string, string) {
	switch provider {
	case "claude":
		return opts.ClaudeEndpointURL, opts.ClaudeEndpointHost
	case "gemini":
		return opts.GeminiEndpointURL, opts.GeminiEndpointHost
	default:
		return opts.CodexEndpointURL, opts.CodexEndpointHost
	}
}

func commandExists(opts Selection, name string) bool {
	if opts.CommandExists != nil {
		return opts.CommandExists(name)
	}
	_, err := osexec.LookPath(name)
	return err == nil
}

// DetectHostAgent infers whether scafld is being driven by Codex or Claude.
// SCAFLD_HOST_AGENT can be set to codex or claude when the host does not expose
// a recognizable environment marker.
func DetectHostAgent(environ []string) string {
	for _, entry := range environ {
		key, value, ok := strings.Cut(entry, "=")
		if ok && strings.EqualFold(key, "SCAFLD_HOST_AGENT") {
			return normalizeHostAgent(value)
		}
	}
	return DetectHostAgentMarker(environ)
}

// DetectHostAgentMarker infers the host vendor from genuine environment markers
// only, ignoring the self-declared SCAFLD_HOST_AGENT. The gate uses this for the
// independence stamp and recorded host vendor so a host cannot manufacture a
// cross_vendor classification by lying about which agent is driving it. It
// returns "" when no marker is present, which classifies as isolation_only.
func DetectHostAgentMarker(environ []string) string {
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(strings.TrimSpace(key))
		switch {
		case codexHostEnvKey(upper):
			return HostAgentCodex
		case claudeHostEnvKey(upper):
			return HostAgentClaude
		}
	}
	return ""
}

// codexHostEnvKey reports whether key is a genuine running-Codex session marker,
// not Codex auth or config. CODEX_HOME, CODEX_API_*, and OPENAI_* are credentials
// or paths that are present whenever Codex is merely the reviewer, so treating
// them as a host signal would let a Claude host overclaim cross-vendor independence.
func codexHostEnvKey(key string) bool {
	switch {
	case key == "CODEX_THREAD_ID" || strings.HasPrefix(key, "CODEX_THREAD"):
		return true
	case key == "CODEX_SESSION_ID" || strings.HasPrefix(key, "CODEX_SESSION"):
		return true
	case key == "CODEX_SANDBOX" || strings.HasPrefix(key, "CODEX_SANDBOX"):
		return true
	default:
		return false
	}
}

func claudeHostEnvKey(key string) bool {
	switch {
	case key == "CLAUDECODE" || strings.HasPrefix(key, "CLAUDECODE_"):
		return true
	case key == "CLAUDE_CODE" || strings.HasPrefix(key, "CLAUDE_CODE_"):
		return true
	case key == "CLAUDE_SESSION_ID" || strings.HasPrefix(key, "CLAUDE_SESSION_"):
		return true
	default:
		return false
	}
}

func normalizeHostAgent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.Contains(value, "codex"):
		return HostAgentCodex
	case strings.Contains(value, "claude"):
		return HostAgentClaude
	case strings.Contains(value, "gemini"):
		return HostAgentGemini
	default:
		return ""
	}
}

func normalizeReviewerProvider(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "codex":
		return HostAgentCodex
	case "claude":
		return HostAgentClaude
	case "gemini":
		return HostAgentGemini
	default:
		return ""
	}
}

func classifyIndependence(hostVendor, reviewerVendor string) Independence {
	host := normalizeHostAgent(hostVendor)
	reviewer := normalizeReviewerProvider(reviewerVendor)
	if host != "" && reviewer != "" && host != reviewer {
		return Independence{Level: IndependenceCrossVendor, Distinct: true}
	}
	return Independence{Level: IndependenceIsolationOnly, Distinct: false}
}

func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
