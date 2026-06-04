package providers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
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
	Provider           string
	Command            string
	Binary             string
	Model              string
	CodexModel         string
	ClaudeModel        string
	GeminiModel        string
	CodexBinary        string
	ClaudeBinary       string
	GeminiBinary       string
	CodexEndpointURL   string
	ClaudeEndpointURL  string
	GeminiEndpointURL  string
	CodexEndpointHost  string
	ClaudeEndpointHost string
	GeminiEndpointHost string
	CWD                string
	Runner             Runner
	Timeout            time.Duration
	Idle               time.Duration
	FallbackPolicy     string
	HostAgent          string
	CommandExists      func(string) bool
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
		return ClaudeProvider{Binary: first(opts.Binary, opts.ClaudeBinary), Model: first(opts.Model, opts.ClaudeModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	case "codex":
		return CodexProvider{Binary: first(opts.Binary, opts.CodexBinary), Model: first(opts.Model, opts.CodexModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
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
		return ClaudeProvider{Binary: first(opts.Binary, opts.ClaudeBinary), Model: first(opts.Model, opts.ClaudeModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	case "codex":
		return CodexProvider{Binary: first(opts.Binary, opts.CodexBinary), Model: first(opts.Model, opts.CodexModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
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
	codexHome := filepath.Join(sandboxHome, ".codex")
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		return nil, fmt.Errorf("create sandbox codex auth home: %w", err)
	}
	if source != "" {
		data, err := os.ReadFile(source)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read codex auth: %w", err)
		}
		if err == nil {
			// Copy ONLY auth.json; the host Codex memory, sessions, and config never
			// enter the sandbox.
			if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), data, 0o600); err != nil {
				return nil, fmt.Errorf("write sandbox codex auth: %w", err)
			}
		}
	}
	// Always override CODEX_HOME with the clean sandbox home (it sorts after the
	// host entry in ScrubProviderEnv's map merge), so the reviewer never reads the
	// host Codex home even when the host sets CODEX_HOME directly.
	return []string{"CODEX_HOME=" + codexHome}, nil
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
		return ClaudeProvider{Binary: first(opts.Binary, opts.ClaudeBinary), Model: first(opts.Model, opts.ClaudeModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	case "gemini":
		return GeminiProvider{Binary: first(opts.Binary, opts.GeminiBinary), Model: first(opts.Model, opts.GeminiModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	default:
		return CodexProvider{Binary: first(opts.Binary, opts.CodexBinary), Model: first(opts.Model, opts.CodexModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	}
}

func agentProviderFor(provider string, opts Selection) Agent {
	switch provider {
	case "claude":
		return ClaudeProvider{Binary: first(opts.Binary, opts.ClaudeBinary), Model: first(opts.Model, opts.ClaudeModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	case "gemini":
		return GeminiProvider{Binary: first(opts.Binary, opts.GeminiBinary), Model: first(opts.Model, opts.GeminiModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	default:
		return CodexProvider{Binary: first(opts.Binary, opts.CodexBinary), Model: first(opts.Model, opts.CodexModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}
	}
}

func receiptGradeAgentProviderFor(provider string, opts Selection, binary ReceiptGradeBinary, env ProviderEnvResult, facts RuntimeFacts, argsPolicy SandboxArgsPolicy) Agent {
	readRoots := append([]string(nil), argsPolicy.ReadRoots...)
	switch provider {
	case "claude":
		return ClaudeProvider{Binary: binary.Path, Model: first(opts.Model, opts.ClaudeModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle, Env: env.Env, EnvMode: execution.EnvModeExact, BinarySHA256: facts.BinarySHA256, EndpointHost: facts.EndpointHost, ReadRoots: readRoots, MemoryAutoloadDisabled: argsPolicy.MemoryAutoloadDisabled, SandboxPolicy: facts.SandboxPolicy}
	case "gemini":
		return GeminiProvider{Binary: binary.Path, Model: first(opts.Model, opts.GeminiModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle, Env: env.Env, EnvMode: execution.EnvModeExact, BinarySHA256: facts.BinarySHA256, EndpointHost: facts.EndpointHost, ReadRoots: readRoots, MemoryAutoloadDisabled: argsPolicy.MemoryAutoloadDisabled, SandboxPolicy: facts.SandboxPolicy}
	default:
		return CodexProvider{Binary: binary.Path, Model: first(opts.Model, opts.CodexModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle, Env: env.Env, EnvMode: execution.EnvModeExact, BinarySHA256: facts.BinarySHA256, EndpointHost: facts.EndpointHost, ReadRoots: readRoots, MemoryAutoloadDisabled: argsPolicy.MemoryAutoloadDisabled, SandboxPolicy: facts.SandboxPolicy}
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

// LocalProvider emits deterministic local review dossiers for development smoke tests.
type LocalProvider struct {
	Messages []string
}

// InvokeAgent returns a deterministic local payload for development smoke tests.
func (p LocalProvider) InvokeAgent(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	var lines []string
	for _, msg := range p.Messages {
		if err := ctx.Err(); err != nil {
			return AgentResponse{}, err
		}
		lines = append(lines, msg)
	}
	text := strings.Join(lines, "\n")
	if strings.TrimSpace(text) == "" {
		switch req.SchemaName {
		case "HardenDossier":
			text = localHardenDossier()
		default:
			text = `{"type":"dossier","dossier":{"verdict":"pass","mode":"discover","summary":"Local provider smoke review passed.","findings":[],"attack_log":[{"target":"local provider","attack":"deterministic smoke review","result":"clean"}],"budget":{"actual_attack_angles":1,"depth":"local"}}}`
		}
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return AgentResponse{Text: text, Provider: "local", OutputFormat: "local.fixture"}, nil
}

// Invoke returns a dossier from configured local messages.
func (p LocalProvider) Invoke(ctx context.Context, req review.Request) (review.Dossier, error) {
	return invokeReviewAgent(ctx, p, req, review.ParseText)
}

// CommandProvider invokes an operator-supplied review command.
type CommandProvider struct {
	Command     string
	CWD         string
	Env         []string
	Runner      Runner
	Timeout     time.Duration
	IdleTimeout time.Duration
}

type receiptGradeSandboxAgent struct {
	Agent   Agent
	sandbox EvidenceSandbox
	facts   RuntimeFacts
}

func (a receiptGradeSandboxAgent) InvokeAgent(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	if a.sandbox.Cleanup != nil {
		defer a.sandbox.Cleanup()
	}
	resp, err := a.Agent.InvokeAgent(ctx, req)
	return stampRuntimeFacts(resp, a.facts), err
}

// InvokeAgent sends the prompt to the command and returns stdout as the payload.
func (p CommandProvider) InvokeAgent(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	if p.Runner == nil {
		return AgentResponse{}, fmt.Errorf("%w: runner is required", ErrProviderFailed)
	}
	if strings.TrimSpace(p.Command) == "" {
		return AgentResponse{}, fmt.Errorf("%w: command is required", ErrProviderFailed)
	}
	env := append([]string(nil), p.Env...)
	env = append(env, "SCAFLD_TASK_ID="+req.TaskID)
	env = append(env, "SCAFLD_SCHEMA_NAME="+req.SchemaName)
	env = append(env, "SCAFLD_SUBMIT_TOOL="+req.SubmitTool.Name)
	result, err := p.Runner.Run(ctx, execution.Request{
		Command:                p.Command,
		Input:                  req.Prompt,
		CWD:                    p.CWD,
		Env:                    env,
		Timeout:                p.Timeout,
		IdleTimeout:            p.IdleTimeout,
		SuppressProgressStderr: true,
	})
	if err != nil && strings.TrimSpace(result.Stdout) == "" {
		return AgentResponse{}, providerFailedError(result, err)
	}
	return AgentResponse{Text: result.Stdout, Provider: "command", OutputFormat: "command.stdout", Result: result, RunErr: err}, nil
}

// Invoke sends the review prompt to the command and parses stdout as a dossier.
func (p CommandProvider) Invoke(ctx context.Context, req review.Request) (review.Dossier, error) {
	return invokeReviewAgent(ctx, p, req, review.ParseText)
}

// ClaudeProvider invokes Claude with a restricted read-only toolset and a
// scafld-owned MCP submit tool for the final dossier.
type ClaudeProvider struct {
	Binary                 string
	Model                  string
	SessionID              string
	ScafldBinary           string
	SubmissionPath         string
	CWD                    string
	Env                    []string
	EnvMode                execution.EnvMode
	BinarySHA256           string
	EndpointHost           string
	ReadRoots              []string
	MemoryAutoloadDisabled bool
	SandboxPolicy          SandboxPolicy
	Runner                 Runner
	Timeout                time.Duration
	IdleTimeout            time.Duration
}

// InvokeAgent sends the prompt to Claude and reads the scafld MCP submission.
func (p ClaudeProvider) InvokeAgent(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	if p.Runner == nil {
		return AgentResponse{}, fmt.Errorf("%w: runner is required", ErrProviderFailed)
	}
	sessionID := p.SessionID
	if sessionID == "" {
		sessionID = newUUID()
	}
	submissionPath := p.SubmissionPath
	cleanup := func() {}
	if submissionPath == "" {
		file, err := os.CreateTemp("", "scafld-claude-"+safeSchemaName(req.SchemaName)+"-*.json")
		if err != nil {
			return AgentResponse{}, fmt.Errorf("%w: create submission file: %v", ErrProviderFailed, err)
		}
		submissionPath = file.Name()
		_ = file.Close()
		cleanup = func() { _ = os.Remove(submissionPath) }
	}
	defer cleanup()
	tool := req.SubmitTool
	if strings.TrimSpace(tool.Name) == "" {
		tool = reviewSubmitTool()
	}
	result, err := p.Runner.Run(ctx, execution.Request{
		Args:                   ClaudeArgs(binaryOrDefault(p.Binary, "claude"), p.Model, sessionID, ClaudeMCPConfig(scafldBinaryOrDefault(p.ScafldBinary), submissionPath, tool), tool, p.ReadRoots),
		Input:                  req.Prompt,
		CWD:                    p.CWD,
		Env:                    p.Env,
		EnvMode:                p.EnvMode,
		Timeout:                p.Timeout,
		IdleTimeout:            p.IdleTimeout,
		SuppressProgressStderr: true,
		StdoutEventInspector:   ClaudeEventName,
	})
	provenance := extractClaudeProvenance(result.Stdout)
	data, readErr := os.ReadFile(filepath.Clean(submissionPath))
	if readErr != nil && !os.IsNotExist(readErr) {
		return AgentResponse{}, fmt.Errorf("%w: read submission: %v", ErrProviderFailed, readErr)
	}
	body := strings.TrimSpace(string(data))
	if body == "" {
		return AgentResponse{}, providerFailedError(result, fmt.Errorf("provider produced no submission; Claude must call %s exactly once and final text is ignored", tool.Name))
	}
	return AgentResponse{
		Text:          body,
		Provider:      "claude",
		Model:         provenance.Model,
		SessionID:     provenance.SessionID,
		OutputFormat:  "claude.mcp_" + tool.Name,
		BinarySHA256:  p.BinarySHA256,
		EndpointHost:  p.EndpointHost,
		SandboxPolicy: p.SandboxPolicy,
		EventSummary:  eventSummary(result.StdoutEvents, provenance.Events),
		Result:        result,
		RunErr:        err,
	}, nil
}

// Invoke sends the review prompt to Claude and parses the resulting dossier.
func (p ClaudeProvider) Invoke(ctx context.Context, req review.Request) (review.Dossier, error) {
	return invokeReviewAgent(ctx, p, req, review.ParseText)
}

// CodexProvider invokes Codex in read-only ephemeral review mode.
type CodexProvider struct {
	Binary                 string
	Model                  string
	SchemaPath             string
	OutputPath             string
	CWD                    string
	Env                    []string
	EnvMode                execution.EnvMode
	BinarySHA256           string
	EndpointHost           string
	ReadRoots              []string
	MemoryAutoloadDisabled bool
	SandboxPolicy          SandboxPolicy
	Runner                 Runner
	Timeout                time.Duration
	IdleTimeout            time.Duration
}

// InvokeAgent sends the prompt to Codex and reads its structured output.
func (p CodexProvider) InvokeAgent(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	if p.Runner == nil {
		return AgentResponse{}, fmt.Errorf("%w: runner is required", ErrProviderFailed)
	}
	outputPath := p.OutputPath
	cleanup := func() {}
	if outputPath == "" {
		file, err := os.CreateTemp("", "scafld-codex-"+safeSchemaName(req.SchemaName)+"-*.json")
		if err != nil {
			return AgentResponse{}, fmt.Errorf("%w: create output file: %v", ErrProviderFailed, err)
		}
		outputPath = file.Name()
		_ = file.Close()
		cleanup = func() { _ = os.Remove(outputPath) }
	}
	defer cleanup()
	schemaPath := p.SchemaPath
	cleanupSchema := func() {}
	if schemaPath == "" {
		file, err := os.CreateTemp("", "scafld-"+safeSchemaName(req.SchemaName)+"-schema-*.json")
		if err != nil {
			return AgentResponse{}, fmt.Errorf("%w: create schema file: %v", ErrProviderFailed, err)
		}
		schemaPath = file.Name()
		schemaJSON := first(req.StrictSchemaJSON, req.SchemaJSON)
		if strings.TrimSpace(schemaJSON) == "" {
			schemaJSON = ReviewDossierSchemaJSON()
		}
		if _, err := file.WriteString(schemaJSON); err != nil {
			_ = file.Close()
			return AgentResponse{}, fmt.Errorf("%w: write schema file: %v", ErrProviderFailed, err)
		}
		_ = file.Close()
		cleanupSchema = func() { _ = os.Remove(schemaPath) }
	}
	defer cleanupSchema()
	result, err := p.Runner.Run(ctx, execution.Request{
		Args:                   CodexArgs(binaryOrDefault(p.Binary, "codex"), p.CWD, outputPath, p.Model, schemaPath),
		Input:                  req.Prompt,
		CWD:                    p.CWD,
		Env:                    p.Env,
		EnvMode:                p.EnvMode,
		Timeout:                p.Timeout,
		IdleTimeout:            p.IdleTimeout,
		SuppressProgressStderr: true,
	})
	body := strings.TrimSpace(result.Stdout)
	outputFormat := "codex.stdout"
	if data, readErr := os.ReadFile(filepath.Clean(outputPath)); readErr == nil && strings.TrimSpace(string(data)) != "" {
		body = string(data)
		outputFormat = "codex.output_file"
	}
	return AgentResponse{Text: body, Provider: "codex", OutputFormat: outputFormat, BinarySHA256: p.BinarySHA256, EndpointHost: p.EndpointHost, SandboxPolicy: p.SandboxPolicy, Result: result, RunErr: err}, nil
}

// Invoke sends the review prompt to Codex and parses the resulting dossier.
func (p CodexProvider) Invoke(ctx context.Context, req review.Request) (review.Dossier, error) {
	return invokeReviewAgent(ctx, p, req, review.ParseText)
}

// GeminiProvider invokes Gemini CLI in read-only plan mode with a scafld-owned
// MCP submit tool for the final dossier.
type GeminiProvider struct {
	Binary                 string
	Model                  string
	ScafldBinary           string
	SubmissionPath         string
	SettingsPath           string
	PolicyPath             string
	CWD                    string
	Env                    []string
	EnvMode                execution.EnvMode
	BinarySHA256           string
	EndpointHost           string
	ReadRoots              []string
	MemoryAutoloadDisabled bool
	SandboxPolicy          SandboxPolicy
	Runner                 Runner
	Timeout                time.Duration
	IdleTimeout            time.Duration
}

// InvokeAgent sends the prompt to Gemini and reads the scafld MCP submission.
func (p GeminiProvider) InvokeAgent(ctx context.Context, req AgentRequest) (AgentResponse, error) {
	if p.Runner == nil {
		return AgentResponse{}, fmt.Errorf("%w: runner is required", ErrProviderFailed)
	}
	submissionPath := p.SubmissionPath
	cleanupSubmission := func() {}
	if submissionPath == "" {
		file, err := os.CreateTemp("", "scafld-gemini-"+safeSchemaName(req.SchemaName)+"-*.json")
		if err != nil {
			return AgentResponse{}, fmt.Errorf("%w: create submission file: %v", ErrProviderFailed, err)
		}
		submissionPath = file.Name()
		_ = file.Close()
		cleanupSubmission = func() { _ = os.Remove(submissionPath) }
	}
	defer cleanupSubmission()
	tool := req.SubmitTool
	if strings.TrimSpace(tool.Name) == "" {
		tool = reviewSubmitTool()
	}
	settingsPath := p.SettingsPath
	cleanupSettings := func() {}
	if settingsPath == "" {
		file, err := os.CreateTemp("", "scafld-gemini-settings-*.json")
		if err != nil {
			return AgentResponse{}, fmt.Errorf("%w: create Gemini settings file: %v", ErrProviderFailed, err)
		}
		settingsPath = file.Name()
		_ = file.Close()
		cleanupSettings = func() { _ = os.Remove(settingsPath) }
	}
	defer cleanupSettings()
	if err := os.WriteFile(filepath.Clean(settingsPath), []byte(GeminiSettingsJSON(scafldBinaryOrDefault(p.ScafldBinary), submissionPath, tool, p.SandboxPolicy)), 0o600); err != nil {
		return AgentResponse{}, fmt.Errorf("%w: write Gemini settings file: %v", ErrProviderFailed, err)
	}
	policyPath := p.PolicyPath
	cleanupPolicy := func() {}
	if policyPath == "" {
		file, err := os.CreateTemp("", "scafld-gemini-policy-*.toml")
		if err != nil {
			return AgentResponse{}, fmt.Errorf("%w: create Gemini policy file: %v", ErrProviderFailed, err)
		}
		policyPath = file.Name()
		_ = file.Close()
		cleanupPolicy = func() { _ = os.Remove(policyPath) }
	}
	defer cleanupPolicy()
	if err := os.WriteFile(filepath.Clean(policyPath), []byte(GeminiPolicyTOML(tool)), 0o600); err != nil {
		return AgentResponse{}, fmt.Errorf("%w: write Gemini policy file: %v", ErrProviderFailed, err)
	}
	env := append([]string(nil), p.Env...)
	env = append(env, "GEMINI_CLI_SYSTEM_SETTINGS_PATH="+settingsPath)
	result, err := p.Runner.Run(ctx, execution.Request{
		Args:                   GeminiArgs(binaryOrDefault(p.Binary, "gemini"), p.Model, tool, policyPath),
		Input:                  GeminiPrompt(req.Prompt, tool),
		CWD:                    p.CWD,
		Env:                    env,
		EnvMode:                p.EnvMode,
		Timeout:                p.Timeout,
		IdleTimeout:            p.IdleTimeout,
		SuppressProgressStderr: true,
		StdoutEventInspector:   GeminiEventName,
	})
	data, readErr := os.ReadFile(filepath.Clean(submissionPath))
	if readErr != nil && !os.IsNotExist(readErr) {
		return AgentResponse{}, fmt.Errorf("%w: read submission: %v", ErrProviderFailed, readErr)
	}
	body := strings.TrimSpace(string(data))
	if body == "" {
		return AgentResponse{}, providerFailedError(result, fmt.Errorf("provider produced no submission; Gemini must call mcp_scafld_%s exactly once and final text is ignored", tool.Name))
	}
	return AgentResponse{
		Text:          body,
		Provider:      "gemini",
		Model:         extractGeminiModel(result.Stdout),
		OutputFormat:  "gemini.mcp_" + tool.Name,
		BinarySHA256:  p.BinarySHA256,
		EndpointHost:  p.EndpointHost,
		SandboxPolicy: p.SandboxPolicy,
		EventSummary:  eventSummary(result.StdoutEvents, nil),
		Result:        result,
		RunErr:        err,
	}, nil
}

// Invoke sends the review prompt to Gemini and parses the resulting dossier.
func (p GeminiProvider) Invoke(ctx context.Context, req review.Request) (review.Dossier, error) {
	return invokeReviewAgent(ctx, p, req, review.ParseText)
}

// ClaudeArgs builds the argv for restricted Claude execution. readRoots scopes
// file access to the evidence sandbox via --add-dir; the reviewer also runs with
// CWD set to that root and a sandbox HOME so no other directory or user-global
// memory is reachable.
func ClaudeArgs(binary string, model string, sessionID string, mcpConfig string, tool SubmitTool, readRoots []string) []string {
	toolName := strings.TrimSpace(tool.Name)
	if toolName == "" {
		toolName = "submit_review"
	}
	args := []string{
		binary,
		"-p",
		"--output-format",
		"stream-json",
		"--verbose",
		"--include-partial-messages",
		"--no-session-persistence",
		"--disable-slash-commands",
		"--no-chrome",
		"--tools",
		"Read,Grep,Glob",
		"--allowedTools",
		"Read,Grep,Glob,mcp__scafld__" + toolName,
		"--disallowedTools",
		"Agent,Task,Bash,Edit,MultiEdit,Write,NotebookEdit",
		"--mcp-config",
		mcpConfig,
		"--strict-mcp-config",
		"--session-id",
		sessionID,
	}
	for _, root := range readRoots {
		if strings.TrimSpace(root) != "" {
			args = append(args, "--add-dir", root)
		}
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	return args
}

// ClaudeMCPConfig returns the single-tool MCP config used by the Claude provider.
func ClaudeMCPConfig(scafldBinary string, submissionPath string, tool SubmitTool) string {
	command := strings.TrimSpace(tool.Command)
	if command == "" {
		command = "review-submit-stdio"
	}
	config := map[string]any{
		"mcpServers": map[string]any{
			"scafld": map[string]any{
				"command": scafldBinary,
				"args":    []string{command, "--out", submissionPath},
			},
		},
	}
	data, err := json.Marshal(config)
	if err != nil {
		return `{"mcpServers":{}}`
	}
	return string(data)
}

// GeminiArgs builds the argv for restricted Gemini execution.
func GeminiArgs(binary string, model string, tool SubmitTool, policyPath string) []string {
	toolName := strings.TrimSpace(tool.Name)
	if toolName == "" {
		toolName = "submit_review"
	}
	args := []string{
		binary,
		"--skip-trust",
		"--approval-mode",
		"plan",
		"--output-format",
		"stream-json",
		"--allowed-mcp-server-names",
		"scafld",
		"--policy",
		policyPath,
		"--prompt",
		"",
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	return args
}

// GeminiPolicyTOML allows exactly the scafld submit tool in Gemini plan mode.
func GeminiPolicyTOML(tool SubmitTool) string {
	toolName := strings.TrimSpace(tool.Name)
	if toolName == "" {
		toolName = "submit_review"
	}
	return `[[rule]]
mcpName = "scafld"
toolName = "` + toolName + `"
decision = "allow"
priority = 900
modes = ["plan"]
interactive = false
`
}

// GeminiSettingsJSON returns an isolated single-server MCP configuration for
// Gemini CLI. It is passed through GEMINI_CLI_SYSTEM_SETTINGS_PATH so scafld does
// not mutate project or user Gemini settings during review.
func GeminiSettingsJSON(scafldBinary string, submissionPath string, tool SubmitTool, policyOpt ...SandboxPolicy) string {
	command := strings.TrimSpace(tool.Command)
	if command == "" {
		command = "review-submit-stdio"
	}
	config := map[string]any{
		"general": map[string]any{
			"defaultApprovalMode": "plan",
		},
		"mcp": map[string]any{
			"allowed": []string{"scafld"},
		},
		"mcpServers": map[string]any{
			"scafld": map[string]any{
				"command":      scafldBinary,
				"args":         []string{command, "--out", submissionPath},
				"includeTools": []string{tool.Name},
				"trust":        true,
				"timeout":      30000,
			},
		},
		"security": map[string]any{
			"disableYoloMode":    true,
			"disableAlwaysAllow": true,
		},
	}
	if len(policyOpt) > 0 && len(policyOpt[0].ReadRoots) > 0 {
		// context.includeDirectories is Gemini's real read-scope setting. Memory
		// autoload is suppressed by the sandbox HOME/XDG override and the clean
		// CWD, not a fabricated key.
		config["context"] = map[string]any{
			"includeDirectories": policyOpt[0].ReadRoots,
		}
	}
	data, err := json.Marshal(config)
	if err != nil {
		return `{"mcpServers":{}}`
	}
	return string(data)
}

// GeminiPrompt adds the provider-specific MCP tool name Gemini exposes for the
// scafld submit channel.
func GeminiPrompt(prompt string, tool SubmitTool) string {
	toolName := strings.TrimSpace(tool.Name)
	if toolName == "" {
		toolName = "submit_review"
	}
	return strings.TrimSpace(prompt) + "\n\nProvider submit channel: Gemini exposes the scafld submit tool as `mcp_scafld_" + toolName + "`. Call that tool exactly once with the final dossier. Do not emit final prose or JSON text."
}

// CodexArgs builds the argv for read-only Codex review execution.
func CodexArgs(binary string, root string, outputPath string, model string, schemaPath string) []string {
	args := []string{
		binary,
		"exec",
		"--sandbox",
		"read-only",
		"--skip-git-repo-check",
		"--cd",
		root,
		"--ephemeral",
		"--ignore-user-config",
		"--ignore-rules",
		"--color",
		"never",
		"--output-last-message",
		outputPath,
	}
	if schemaPath != "" {
		args = append(args, "--output-schema", schemaPath)
	}
	if model != "" {
		args = append(args, "-m", model)
	}
	return args
}

// SelectHarden returns the configured harden provider implementation.
func SelectHarden(opts Selection) (interface {
	Invoke(context.Context, coreharden.Request) (coreharden.Dossier, error)
}, error) {
	agent, err := SelectAgent(opts)
	if err != nil {
		return nil, err
	}
	return HardenProvider{Agent: agent}, nil
}

// HardenProvider adapts shared agent transport to the harden dossier protocol.
type HardenProvider struct {
	Agent Agent
}

// Invoke returns a typed harden dossier from the shared provider transport.
func (p HardenProvider) Invoke(ctx context.Context, req coreharden.Request) (coreharden.Dossier, error) {
	agent := p.Agent
	if agent == nil {
		return coreharden.Dossier{}, fmt.Errorf("%w: provider is required", ErrProviderFailed)
	}
	resp, err := agent.InvokeAgent(ctx, AgentRequest{
		TaskID:           req.TaskID,
		Prompt:           req.Prompt,
		SchemaName:       "HardenDossier",
		SchemaJSON:       coreharden.DossierSchemaJSON(),
		StrictSchemaJSON: coreharden.StrictDossierSchemaJSON(),
		SubmitTool:       hardenSubmitTool(),
	})
	if err != nil {
		return coreharden.Dossier{}, err
	}
	dossier, dossierErr := hardenDossierFromProviderResult(resp.Result, resp.RunErr, resp.Text)
	if dossierErr != nil {
		return coreharden.Dossier{}, dossierErr
	}
	dossier.Provider = first(resp.Provider, dossier.Provider)
	dossier.Model = first(resp.Model, dossier.Model)
	dossier.SessionID = first(resp.SessionID, dossier.SessionID)
	dossier.OutputFormat = first(resp.OutputFormat, dossier.OutputFormat)
	dossier.EventSummary = mergeEventSummary(dossier.EventSummary, resp.EventSummary)
	return dossier, nil
}

func invokeReviewAgent(ctx context.Context, agent Agent, req review.Request, parse func(string) (review.Dossier, error)) (review.Dossier, error) {
	resp, err := agent.InvokeAgent(ctx, AgentRequest{
		TaskID:           req.TaskID,
		Prompt:           req.Prompt,
		SchemaName:       "ReviewDossier",
		SchemaJSON:       review.DossierSchemaJSON(),
		StrictSchemaJSON: ReviewDossierSchemaJSON(),
		SubmitTool:       reviewSubmitTool(),
	})
	if err != nil {
		return review.Dossier{}, err
	}
	dossier, dossierErr := dossierFromProviderResult(resp.Result, resp.RunErr, resp.Text, parse)
	if dossierErr != nil {
		return review.Dossier{}, dossierErr
	}
	dossier.Provider = first(resp.Provider, dossier.Provider)
	dossier.Model = first(resp.Model, dossier.Model)
	dossier.SessionID = first(resp.SessionID, dossier.SessionID)
	dossier.OutputFormat = first(resp.OutputFormat, dossier.OutputFormat)
	dossier.EventSummary = mergeEventSummary(dossier.EventSummary, resp.EventSummary)
	return dossier, nil
}

func dossierFromProviderResult(result execution.Result, runErr error, text string, parse func(string) (review.Dossier, error)) (review.Dossier, error) {
	if runErr != nil && strings.TrimSpace(text) == "" {
		return review.Dossier{}, providerFailedError(result, runErr)
	}
	dossier, parseErr := parse(text)
	if parseErr != nil {
		if runErr != nil {
			return review.Dossier{}, providerFailedError(result, runErr)
		}
		if result.DiagnosticPath != "" {
			return review.Dossier{}, providerFailedError(result, parseErr)
		}
		if result.ExitCode != 0 {
			return review.Dossier{}, providerFailedError(result, fmt.Errorf("exit code %d", result.ExitCode))
		}
		return review.Dossier{}, parseErr
	}
	if runErr != nil {
		return review.Dossier{}, providerFailedError(result, runErr)
	}
	if result.ExitCode != 0 && dossier.Verdict != review.VerdictFail {
		return review.Dossier{}, providerFailedError(result, fmt.Errorf("exit code %d", result.ExitCode))
	}
	return dossier, nil
}

func hardenDossierFromProviderResult(result execution.Result, runErr error, text string) (coreharden.Dossier, error) {
	if runErr != nil && strings.TrimSpace(text) == "" {
		return coreharden.Dossier{}, providerFailedError(result, runErr)
	}
	dossier, parseErr := coreharden.ParseText(text)
	if parseErr != nil {
		if runErr != nil {
			return coreharden.Dossier{}, providerFailedError(result, runErr)
		}
		if result.DiagnosticPath != "" {
			return coreharden.Dossier{}, providerFailedError(result, parseErr)
		}
		if result.ExitCode != 0 {
			return coreharden.Dossier{}, providerFailedError(result, fmt.Errorf("exit code %d", result.ExitCode))
		}
		return coreharden.Dossier{}, parseErr
	}
	if runErr != nil {
		return coreharden.Dossier{}, providerFailedError(result, runErr)
	}
	if result.ExitCode != 0 && dossier.Verdict != coreharden.VerdictNeedsRevision {
		return coreharden.Dossier{}, providerFailedError(result, fmt.Errorf("exit code %d", result.ExitCode))
	}
	return dossier, nil
}

func providerFailedError(result execution.Result, cause error) error {
	detail := ""
	if stderr := errorSnippet(result.Stderr); stderr != "" {
		detail += ": " + stderr
	} else if stdout := errorSnippet(result.Stdout); stdout != "" {
		detail += ": " + stdout
	}
	if result.DiagnosticPath != "" {
		detail += " (diagnostic: " + result.DiagnosticPath + ")"
	}
	return providerFailureError{cause: cause, detail: detail, diagnosticPath: result.DiagnosticPath}
}

type providerFailureError struct {
	cause          error
	detail         string
	diagnosticPath string
}

func (e providerFailureError) Error() string {
	return fmt.Sprintf("%v: %v%s", ErrProviderFailed, e.cause, e.detail)
}

func (e providerFailureError) Unwrap() []error {
	return []error{ErrProviderFailed, e.cause}
}

func (e providerFailureError) DiagnosticPath() string {
	return e.diagnosticPath
}

func errorSnippet(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	const max = 1200
	if len(text) <= max {
		return text
	}
	return "... " + text[len(text)-max:]
}

type claudeProvenance struct {
	Model     string
	SessionID string
	Events    map[string]int
}

func extractClaudeProvenance(stdout string) claudeProvenance {
	out := claudeProvenance{Events: map[string]int{}}
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if name := ClaudeEventName(line); name != "" {
			out.Events[name]++
		}
		if event["type"] == "system" && event["subtype"] == "init" {
			out.Model = stringField(event, "model", "model_id", "modelId")
			out.SessionID = stringField(event, "session_id", "sessionId")
		}
	}
	return out
}

// ClaudeEventName extracts a liveness event name from one Claude stream frame.
func ClaudeEventName(line string) string {
	var event map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &event); err != nil {
		return ""
	}
	eventType, _ := event["type"].(string)
	if eventType == "" {
		return ""
	}
	subtype, _ := event["subtype"].(string)
	if subtype != "" {
		return eventType + "." + subtype
	}
	return eventType
}

// GeminiEventName extracts a liveness event name from one Gemini stream frame.
func GeminiEventName(line string) string {
	var event map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &event); err != nil {
		return ""
	}
	eventType := stringField(event, "type", "event", "kind")
	if eventType == "" {
		return ""
	}
	return eventType
}

func extractGeminiModel(stdout string) string {
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if model := stringField(event, "model", "model_id", "modelId"); model != "" {
			return model
		}
		if nested, ok := event["result"].(map[string]any); ok {
			if model := stringField(nested, "model", "model_id", "modelId"); model != "" {
				return model
			}
		}
		if nested, ok := event["response"].(map[string]any); ok {
			if model := stringField(nested, "model", "model_id", "modelId"); model != "" {
				return model
			}
		}
	}
	return ""
}

func stringField(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok {
			return value
		}
	}
	return ""
}

func eventSummary(primary map[string]int, fallback map[string]int) map[string]int {
	source := fallback
	if len(primary) > 0 {
		source = primary
	}
	merged := make(map[string]int, len(source))
	for key, value := range source {
		merged[key] = value
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func mergeEventSummary(primary map[string]int, fallback map[string]int) map[string]int {
	if len(primary) > 0 {
		return eventSummary(primary, nil)
	}
	return eventSummary(fallback, nil)
}

func reviewSubmitTool() SubmitTool {
	return SubmitTool{
		Name:        "submit_review",
		Title:       "Submit scafld review",
		Description: "Submit the final scafld ReviewDossier. Call exactly once after completing the read-only adversarial review.",
		Command:     "review-submit-stdio",
	}
}

func hardenSubmitTool() SubmitTool {
	return SubmitTool{
		Name:        "submit_harden",
		Title:       "Submit scafld hardening",
		Description: "Submit the final scafld HardenDossier. Call exactly once after stress-testing the draft spec. The checks array is a fixed six-row form: path audit, command audit, scope/migration audit, acceptance timing audit, rollback/repair audit, design challenge. Fill grounded_in, result, and evidence for every check.",
		Command:     "harden-submit-stdio",
	}
}

func localHardenDossier() string {
	return `{"verdict":"pass","summary":"Local provider smoke hardening passed.","checks":[{"name":"path audit","grounded_in":"spec_gap:Context","result":"passed","evidence":"Local smoke provider records required harden checks."},{"name":"command audit","grounded_in":"spec_gap:Acceptance","result":"passed","evidence":"Local smoke provider records required harden checks."},{"name":"scope/migration audit","grounded_in":"spec_gap:Scope","result":"passed","evidence":"Local smoke provider records required harden checks."},{"name":"acceptance timing audit","grounded_in":"spec_gap:Acceptance","result":"passed","evidence":"Local smoke provider records required harden checks."},{"name":"rollback/repair audit","grounded_in":"spec_gap:Rollback","result":"passed","evidence":"Local smoke provider records required harden checks."},{"name":"design challenge","grounded_in":"spec_gap:Summary","result":"passed","evidence":"Local smoke provider records required harden checks."}],"issues":[],"attack_log":[{"target":"local provider","attack":"deterministic smoke hardening","result":"clean"}]}`
}

func safeSchemaName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "dossier"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "dossier"
	}
	return out
}

func binaryOrDefault(binary string, fallback string) string {
	if strings.TrimSpace(binary) == "" {
		return fallback
	}
	return binary
}

func scafldBinaryOrDefault(binary string) string {
	if strings.TrimSpace(binary) != "" {
		return binary
	}
	if executable, err := os.Executable(); err == nil && strings.TrimSpace(executable) != "" {
		return executable
	}
	return "scafld"
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
