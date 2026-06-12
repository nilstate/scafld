package providers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
	"github.com/nilstate/scafld/v2/internal/core/review"
)

func TestProviderContract(t *testing.T) {
	t.Parallel()
	dossier, err := (LocalProvider{Messages: []string{dossierFrame(review.VerdictFail)}}).Invoke(context.Background(), review.Request{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if dossier.Verdict != "fail" || dossier.Provider != "local" || len(dossier.Findings) != 1 {
		t.Fatalf("dossier = %+v", dossier)
	}
}

func TestHardenProviderContract(t *testing.T) {
	t.Parallel()

	provider, err := SelectHarden(Selection{Provider: "local"})
	if err != nil {
		t.Fatal(err)
	}
	dossier, err := provider.Invoke(context.Background(), coreharden.Request{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if coreharden.VerdictFromDossier(dossier) != coreharden.VerdictPass || dossier.Provider != "local" || len(dossier.Observations) != len(coreharden.RequiredDimensions) {
		t.Fatalf("dossier = %+v", dossier)
	}
}

func TestAutoProviderPrefersOppositeHostAgent(t *testing.T) {
	t.Parallel()

	selected, err := AutoProviderInfo(Selection{
		Provider:     "auto",
		HostAgent:    HostAgentCodex,
		CodexBinary:  "/opt/bin/codex",
		ClaudeBinary: "/opt/bin/claude",
		CodexModel:   "gpt-review",
		ClaudeModel:  "opus-review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != "claude" || selected.Model != "opus-review" {
		t.Fatalf("selected = %+v, want claude opposite Codex host", selected)
	}

	selected, err = AutoProviderInfo(Selection{
		Provider:     "auto",
		HostAgent:    HostAgentClaude,
		CodexBinary:  "/opt/bin/codex",
		ClaudeBinary: "/opt/bin/claude",
		CodexModel:   "gpt-review",
		ClaudeModel:  "opus-review",
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != "codex" || selected.Model != "gpt-review" {
		t.Fatalf("selected = %+v, want codex opposite Claude host", selected)
	}
}

func TestAutoProviderFallsBackToSameHostAgent(t *testing.T) {
	t.Parallel()

	selected, err := AutoProviderInfo(Selection{
		Provider:      "auto",
		HostAgent:     HostAgentCodex,
		CodexBinary:   "/opt/bin/codex",
		CodexModel:    "gpt-review",
		CommandExists: func(string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != "codex" || selected.Model != "gpt-review" {
		t.Fatalf("selected = %+v, want codex fallback when Claude is unavailable", selected)
	}
}

func TestAutoProviderIncludesGeminiAsSecondaryExternalChallenger(t *testing.T) {
	t.Parallel()

	selected, err := AutoProviderInfo(Selection{
		Provider:      "auto",
		HostAgent:     HostAgentCodex,
		GeminiBinary:  "/opt/bin/gemini",
		GeminiModel:   "gemini-review",
		CommandExists: func(string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != "gemini" || selected.Model != "gemini-review" {
		t.Fatalf("selected = %+v, want gemini after missing Claude opposite Codex host", selected)
	}
}

func TestAutoProviderFallbackPolicyDisableStillAllowsGeminiChallenger(t *testing.T) {
	t.Parallel()

	selected, err := AutoProviderInfo(Selection{
		Provider:       "auto",
		HostAgent:      HostAgentCodex,
		CodexBinary:    "/opt/bin/codex",
		GeminiBinary:   "/opt/bin/gemini",
		FallbackPolicy: "disable",
		CommandExists:  func(string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != "gemini" {
		t.Fatalf("selected = %+v, want Gemini independent challenger", selected)
	}
}

func TestAutoProviderBinaryDoesNotOverrideOppositeAgentPreference(t *testing.T) {
	t.Parallel()

	selected, err := AutoProviderInfo(Selection{
		Provider:      "auto",
		HostAgent:     HostAgentCodex,
		Binary:        "/custom/bin/reviewer",
		CodexModel:    "gpt-review",
		ClaudeModel:   "opus-review",
		CommandExists: func(string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != "claude" || selected.Model != "opus-review" {
		t.Fatalf("selected = %+v, want generic binary applied after opposite-agent selection", selected)
	}

	provider, err := Select(Selection{
		Provider:      "auto",
		HostAgent:     HostAgentCodex,
		Binary:        "/custom/bin/reviewer",
		CommandExists: func(string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	claude, ok := provider.(ClaudeProvider)
	if !ok {
		t.Fatalf("provider = %T, want ClaudeProvider", provider)
	}
	if claude.Binary != "/custom/bin/reviewer" {
		t.Fatalf("claude binary = %q, want generic override", claude.Binary)
	}
}

func TestAutoProviderFallbackPolicyDisableBlocksMissingOppositeAgent(t *testing.T) {
	t.Parallel()

	_, err := AutoProviderInfo(Selection{
		Provider:       "auto",
		HostAgent:      HostAgentCodex,
		CodexBinary:    "/opt/bin/codex",
		FallbackPolicy: "disable",
		CommandExists:  func(string) bool { return false },
	})
	if err == nil || !strings.Contains(err.Error(), "no independent auto provider found") {
		t.Fatalf("err = %v, want missing opposite provider failure", err)
	}
}

func TestHostMarkerDoesNotLetCodexAuthOverrideClaudeHost(t *testing.T) {
	t.Parallel()

	// A Claude host with Codex reviewer credentials present must stay "claude";
	// otherwise it overclaims cross-vendor independence against a Claude reviewer.
	environ := []string{"CODEX_HOME=/home/u/.codex", "OPENAI_API_KEY=secret", "CLAUDE_CODE_SESSION=abc"}
	if got := DetectHostAgentMarker(environ); got != HostAgentClaude {
		t.Fatalf("DetectHostAgentMarker = %q, want %q (Codex auth must not override a Claude host)", got, HostAgentClaude)
	}
	// Codex auth/config alone, with no genuine session marker, is not a host signal.
	if got := DetectHostAgentMarker([]string{"CODEX_HOME=/home/u/.codex", "OPENAI_API_KEY=secret"}); got != "" {
		t.Fatalf("DetectHostAgentMarker = %q, want empty for Codex auth-only env", got)
	}
	// A genuine Codex session marker is still detected as a Codex host.
	if got := DetectHostAgentMarker([]string{"CODEX_THREAD_ID=abc"}); got != HostAgentCodex {
		t.Fatalf("DetectHostAgentMarker = %q, want %q for a genuine Codex session", got, HostAgentCodex)
	}
}

func TestReceiptGradeCodexHostCodexHomeDoesNotLeak(t *testing.T) {
	t.Parallel()

	hostCodex := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostCodex, "auth.json"), []byte(`{"token":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostCodex, "history.jsonl"), []byte("secret host memory\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sandboxHome := t.TempDir()
	extra, err := receiptGradeExtraAuthEnv("codex", []string{"CODEX_HOME=" + hostCodex}, sandboxHome)
	if err != nil {
		t.Fatal(err)
	}
	// CODEX_HOME is overridden to the clean sandbox home, never the host home.
	want := "CODEX_HOME=" + filepath.Join(sandboxHome, ".codex")
	if len(extra) != 1 || extra[0] != want {
		t.Fatalf("extra env = %v, want %q (host CODEX_HOME must not leak)", extra, want)
	}
	// Only auth.json is copied; host Codex memory never enters the sandbox.
	if _, err := os.Stat(filepath.Join(sandboxHome, ".codex", "auth.json")); err != nil {
		t.Fatalf("auth.json was not copied into the sandbox codex home: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sandboxHome, ".codex", "history.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("host codex memory leaked into the sandbox: %v", err)
	}
}

func TestDetectHostAgent(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		environ []string
		want    string
	}{
		{name: "explicit codex", environ: []string{"SCAFLD_HOST_AGENT=codex"}, want: HostAgentCodex},
		{name: "explicit claude", environ: []string{"SCAFLD_HOST_AGENT=claude"}, want: HostAgentClaude},
		{name: "codex env", environ: []string{"CODEX_THREAD_ID=abc"}, want: HostAgentCodex},
		{name: "claude env", environ: []string{"CLAUDE_CODE_SESSION=abc"}, want: HostAgentClaude},
		{name: "claude api key is not host signal", environ: []string{"CLAUDE_API_KEY=secret"}, want: ""},
		{name: "unknown", environ: []string{"VSCODE_PID=1"}, want: ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := DetectHostAgent(tt.environ); got != tt.want {
				t.Fatalf("DetectHostAgent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassifyIndependenceUnknownHost(t *testing.T) {
	t.Parallel()

	got := classifyIndependence("vscode", "codex")
	if got.Level != IndependenceIsolationOnly || got.Distinct {
		t.Fatalf("independence = %+v, want isolation_only for unknown host", got)
	}
}

func TestClassifyIndependenceEmptyHost(t *testing.T) {
	t.Parallel()

	got := classifyIndependence("", "claude")
	if got.Level != IndependenceIsolationOnly || got.Distinct {
		t.Fatalf("independence = %+v, want isolation_only for empty host", got)
	}
}

func TestClassifyIndependenceGeminiReviewer(t *testing.T) {
	t.Parallel()

	got := classifyIndependence(HostAgentCodex, "gemini")
	if got.Level != IndependenceCrossVendor || !got.Distinct {
		t.Fatalf("independence = %+v, want cross_vendor Gemini reviewer", got)
	}
}

func TestSelectGateReviewerHostOnlyDoesNotReturnStall(t *testing.T) {
	t.Parallel()

	selected, err := SelectGateReviewer(Selection{
		HostAgent:      HostAgentCodex,
		CodexBinary:    "/opt/bin/codex",
		FallbackPolicy: "disable",
		CommandExists:  func(string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != "codex" || selected.Independence.Level != IndependenceIsolationOnly || selected.Independence.Distinct {
		t.Fatalf("selected = %+v, want host provider at isolation_only", selected)
	}
}

func TestSelectGateReviewerUnknownHost(t *testing.T) {
	t.Parallel()

	selected, err := SelectGateReviewer(Selection{
		CodexBinary:   "/opt/bin/codex",
		CommandExists: func(string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != "codex" || selected.Independence.Level != IndependenceIsolationOnly || selected.Independence.Distinct {
		t.Fatalf("selected = %+v, want first available provider at isolation_only", selected)
	}
}

func TestSelectGateReviewerGeminiCrossVendor(t *testing.T) {
	t.Parallel()

	selected, err := SelectGateReviewer(Selection{
		HostAgent:     HostAgentCodex,
		GeminiBinary:  "/opt/bin/gemini",
		CommandExists: func(string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != "gemini" || selected.Independence.Level != IndependenceCrossVendor || !selected.Independence.Distinct {
		t.Fatalf("selected = %+v, want Gemini cross_vendor reviewer", selected)
	}
}

func TestScrubDropsProxy(t *testing.T) {
	t.Parallel()

	result, err := ScrubProviderEnv(ProviderEnvInput{
		Provider:     "codex",
		EndpointHost: "api.openai.com",
		HostEnviron: []string{
			"OPENAI_API_KEY=secret",
			"HTTPS_PROXY=http://proxy.invalid",
			"http_proxy=http://proxy.invalid",
			"NO_PROXY=localhost",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	env := strings.Join(result.Env, "\n")
	for _, blocked := range []string{"HTTPS_PROXY=", "http_proxy=", "NO_PROXY="} {
		if strings.Contains(env, blocked) {
			t.Fatalf("proxy env %q survived scrub: %v", blocked, result.Env)
		}
	}
	if result.EndpointHost != "api.openai.com" {
		t.Fatalf("endpoint host = %q", result.EndpointHost)
	}
}

func TestScrubPinsEndpoint(t *testing.T) {
	t.Parallel()

	result, err := ScrubProviderEnv(ProviderEnvInput{
		Provider:     "codex",
		EndpointHost: "api.openai.com",
		HostEnviron: []string{
			"OPENAI_API_KEY=secret",
			"OPENAI_BASE_URL=https://api.openai.com/v1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsEnv(result.Env, "OPENAI_BASE_URL=https://api.openai.com/v1") {
		t.Fatalf("pinned endpoint env missing: %v", result.Env)
	}
}

func TestScrubAllowsProviderAuth(t *testing.T) {
	t.Parallel()

	result, err := ScrubProviderEnv(ProviderEnvInput{
		Provider: "claude",
		HostEnviron: []string{
			"ANTHROPIC_API_KEY=anthropic",
			"OPENAI_API_KEY=openai",
			"PATH=/bin",
		},
		RequireAuth: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsEnv(result.Env, "ANTHROPIC_API_KEY=anthropic") || containsEnv(result.Env, "OPENAI_API_KEY=openai") {
		t.Fatalf("auth allowlist wrong: %v", result.Env)
	}
}

func TestScrubAllowsCodexHomeAsAuth(t *testing.T) {
	t.Parallel()

	result, err := ScrubProviderEnv(ProviderEnvInput{
		Provider: "codex",
		HostEnviron: []string{
			"CODEX_HOME=/tmp/codex-auth",
			"HOME=/Users/alice",
			"PATH=/bin",
		},
		RequireAuth: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsEnv(result.Env, "CODEX_HOME=/tmp/codex-auth") {
		t.Fatalf("codex auth home missing: %v", result.Env)
	}
}

func TestScrubMemoryHomeOverridesHostAgentMemory(t *testing.T) {
	t.Parallel()

	home := filepath.Join(t.TempDir(), "review-home")
	result, err := ScrubProviderEnv(ProviderEnvInput{
		Provider:   "codex",
		MemoryHome: home,
		HostEnviron: []string{
			"OPENAI_API_KEY=secret",
			"HOME=/Users/alice",
			"XDG_CONFIG_HOME=/Users/alice/.config",
			"PATH=/bin",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !containsEnv(result.Env, "HOME="+home) || !containsEnv(result.Env, "XDG_CONFIG_HOME="+filepath.Join(home, ".config")) {
		t.Fatalf("memory home override missing: %v", result.Env)
	}
	env := strings.Join(result.Env, "\n")
	if strings.Contains(env, "/Users/alice") {
		t.Fatalf("host memory path survived scrub: %v", result.Env)
	}
}

func TestReceiptGradeCodexCopiesOnlyAuthJSONIntoSandboxHome(t *testing.T) {
	t.Parallel()

	hostHome := t.TempDir()
	hostCodex := filepath.Join(hostHome, ".codex")
	if err := os.MkdirAll(filepath.Join(hostCodex, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostCodex, "auth.json"), []byte(`{"token":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostCodex, "config.toml"), []byte("model = \"host\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostCodex, "sessions", "session.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := writeExecutable(t, "codex-reviewer")
	runner := &fakeRunner{result: execution.Result{Stdout: dossierFrame(review.VerdictPass)}}
	agent, facts, err := SelectReceiptGradeAgentWithEvidence(Selection{
		Provider:          "codex",
		CodexBinary:       binary,
		CodexEndpointHost: "api.openai.com",
		Runner:            runner,
	}, []string{"HOME=" + hostHome, "PATH=/bin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, ok := agent.(receiptGradeSandboxAgent)
	if !ok {
		t.Fatalf("agent = %T, want sandbox wrapper", agent)
	}
	defer wrapped.sandbox.Cleanup()
	codex, ok := wrapped.Agent.(CodexProvider)
	if !ok {
		t.Fatalf("agent = %T, want sandboxed codex provider", agent)
	}
	codexHome := envValue(codex.Env, "CODEX_HOME")
	home := envValue(codex.Env, "HOME")
	if codexHome == "" || home == "" || !strings.HasPrefix(codexHome, home+string(filepath.Separator)) {
		t.Fatalf("CODEX_HOME must be under sandbox HOME: codex_home=%q home=%q facts=%+v", codexHome, home, facts)
	}
	if _, err := os.Stat(filepath.Join(codexHome, "auth.json")); err != nil {
		t.Fatalf("sandbox auth.json missing: %v", err)
	}
	for _, forbidden := range []string{"config.toml", filepath.Join("sessions", "session.jsonl")} {
		if _, err := os.Stat(filepath.Join(codexHome, forbidden)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("copied host codex state %s: %v", forbidden, err)
		}
	}
	if strings.Contains(strings.Join(codex.Env, "\n"), hostCodex) {
		t.Fatalf("host codex path leaked into reviewer env: %v", codex.Env)
	}
}

func TestScrubRejectsUnpinnedEndpoint(t *testing.T) {
	t.Parallel()

	result, err := ScrubProviderEnv(ProviderEnvInput{
		Provider:     "codex",
		EndpointHost: "api.openai.com",
		HostEnviron: []string{
			"OPENAI_API_KEY=secret",
			"OPENAI_BASE_URL=https://evil.invalid",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if containsEnv(result.Env, "OPENAI_BASE_URL=https://evil.invalid") {
		t.Fatalf("unpinned endpoint env survived scrub: %v", result.Env)
	}
}

func TestReceiptGradeBinaryPinnedAbsolute(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "codex-reviewer")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveReceiptGradeBinary(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != path || len(got.SHA256) != 64 {
		t.Fatalf("binary = %+v", got)
	}
}

func TestReceiptGradeBinaryRejectsRelative(t *testing.T) {
	t.Parallel()

	if _, err := ResolveReceiptGradeBinary("codex"); err == nil {
		t.Fatal("expected relative/PATH-only binary to fail")
	}
}

func TestReceiptGradeBinaryRejectsMissing(t *testing.T) {
	t.Parallel()

	if _, err := ResolveReceiptGradeBinary(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing binary to fail")
	}
}

func TestReceiptGradeBinaryRejectsNonExecutable(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "codex-reviewer")
	if err := os.WriteFile(path, []byte("not executable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveReceiptGradeBinary(path); err == nil {
		t.Fatal("expected non-executable binary to fail")
	}
}

func TestReceiptGradeAgentUsesExactEnvAndRuntimeFacts(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "codex-reviewer")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	agent, facts, err := SelectReceiptGradeAgent(Selection{
		Provider:          "codex",
		CodexBinary:       path,
		CodexEndpointHost: "api.openai.com",
		Runner:            &fakeRunner{},
	}, []string{
		"OPENAI_API_KEY=secret",
		"HTTPS_PROXY=http://proxy.invalid",
		"OPENAI_BASE_URL=https://api.openai.com/v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Receipt-grade review is always sandboxed, even without byte-bearing evidence,
	// so the provider is wrapped; unwrap it to inspect the exact-env codex provider.
	wrapped, ok := agent.(receiptGradeSandboxAgent)
	if !ok {
		t.Fatalf("agent = %T, want receiptGradeSandboxAgent", agent)
	}
	t.Cleanup(func() {
		if wrapped.sandbox.Cleanup != nil {
			wrapped.sandbox.Cleanup()
		}
	})
	codex, ok := wrapped.Agent.(CodexProvider)
	if !ok {
		t.Fatalf("agent = %T, want CodexProvider", wrapped.Agent)
	}
	if codex.EnvMode != execution.EnvModeExact || facts.BinarySHA256 == "" || codex.BinarySHA256 != facts.BinarySHA256 || facts.EndpointHost != "api.openai.com" {
		t.Fatalf("receipt-grade provider/facts wrong: provider=%+v facts=%+v", codex, facts)
	}
	env := strings.Join(codex.Env, "\n")
	if !strings.Contains(env, "OPENAI_API_KEY=secret") || strings.Contains(env, "HTTPS_PROXY=") {
		t.Fatalf("receipt-grade env wrong: %v", codex.Env)
	}
}

func containsEnv(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func dossierJSON(verdict string) string {
	if verdict == review.VerdictFail {
		return `{"verdict":"fail","mode":"discover","summary":"bug found","findings":[{"id":"bug","severity":"high","blocks_completion":true,"location":{"path":"file.go"},"evidence":"bug","impact":"breaks behavior","validation":"rerun tests","summary":"bug"}],"attack_log":[{"target":"diff","attack":"scan","result":"finding"}],"budget":{"actual_findings":1,"actual_attack_angles":1,"depth":"test"}}`
	}
	return `{"verdict":"pass","mode":"discover","summary":"clean","findings":[],"attack_log":[{"target":"diff","attack":"scan","result":"clean"}],"budget":{"actual_attack_angles":1,"depth":"test"}}`
}

func dossierWithStringLocationJSON() string {
	return `{"verdict":"pass","mode":"discover","summary":"advisory only","findings":[{"id":"advisory","severity":"low","blocks_completion":false,"location":"api/app/services/report/engagement.rb:51-66","evidence":"review note","impact":"minor clarity issue","validation":"optional inspection","summary":"api/app/services/report/engagement.rb:51-66 has an advisory."}],"attack_log":[{"target":"diff","attack":"scan","result":"clean"}],"budget":{"actual_findings":1,"actual_attack_angles":1,"depth":"test"}}`
}

func dossierFrame(verdict string) string {
	return `{"type":"dossier","dossier":` + dossierJSON(verdict) + `}`
}

type fakeRunner struct {
	result execution.Result
	err    error
	req    execution.Request
	onRun  func(execution.Request)
}

func (f *fakeRunner) Run(_ context.Context, req execution.Request) (execution.Result, error) {
	f.req = req
	if f.onRun != nil {
		f.onRun(req)
	}
	return f.result, f.err
}

func writeClaudeSubmission(t *testing.T, req execution.Request, body string) {
	t.Helper()
	_, path := claudeSubmissionCommand(t, req)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func claudeSubmissionCommand(t *testing.T, req execution.Request) (string, string) {
	t.Helper()
	var raw string
	for i, arg := range req.Args {
		if arg == "--mcp-config" && i+1 < len(req.Args) {
			raw = req.Args[i+1]
			break
		}
	}
	if raw == "" {
		t.Fatalf("missing --mcp-config: %+v", req.Args)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("mcp config: %v\n%s", err, raw)
	}
	server, ok := cfg.MCPServers["scafld"]
	if !ok {
		t.Fatalf("missing scafld MCP server: %+v", cfg)
	}
	for i, arg := range server.Args {
		if arg == "--out" && i+1 < len(server.Args) {
			return server.Command, server.Args[i+1]
		}
	}
	t.Fatalf("missing --out in mcp args: %+v", server.Args)
	return "", ""
}

func writeGeminiSubmission(t *testing.T, req execution.Request, body string) {
	t.Helper()
	settings := geminiSettingsPath(t, req)
	data, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Args []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("gemini settings: %v\n%s", err, data)
	}
	server, ok := cfg.MCPServers["scafld"]
	if !ok {
		t.Fatalf("missing scafld MCP server: %+v", cfg)
	}
	for i, arg := range server.Args {
		if arg == "--out" && i+1 < len(server.Args) {
			if err := os.WriteFile(server.Args[i+1], []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("missing --out in gemini settings: %s", data)
}

func geminiSettingsPath(t *testing.T, req execution.Request) string {
	t.Helper()
	for _, entry := range req.Env {
		key, value, ok := strings.Cut(entry, "=")
		if ok && key == "GEMINI_CLI_SYSTEM_SETTINGS_PATH" {
			return value
		}
	}
	t.Fatalf("missing GEMINI_CLI_SYSTEM_SETTINGS_PATH env: %+v", req.Env)
	return ""
}

func TestCommandProviderParsesStdoutOnlyAndPassesTimeouts(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: execution.Result{
		ExitCode: 0,
		Stdout:   dossierFrame(review.VerdictFail) + "\n",
		Stderr:   "progress\n",
		Output:   "progress should not be parsed",
	}}
	dossier, err := (CommandProvider{
		Command:     "reviewer",
		CWD:         "/tmp/work",
		Runner:      runner,
		Timeout:     time.Minute,
		IdleTimeout: time.Second,
	}).Invoke(context.Background(), review.Request{TaskID: "task", Prompt: "prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if dossier.Verdict != review.VerdictFail || dossier.Provider != "command" || len(dossier.Findings) != 1 {
		t.Fatalf("dossier = %+v", dossier)
	}
	if runner.req.CWD != "/tmp/work" || runner.req.Timeout != time.Minute || runner.req.IdleTimeout != time.Second || runner.req.Input != "prompt" {
		t.Fatalf("request = %+v", runner.req)
	}
}

func TestProviderTransportStampsAuthoritativeProvider(t *testing.T) {
	t.Parallel()

	text := `{"type":"dossier","dossier":{"verdict":"pass","mode":"discover","summary":"clean","provider":"codex","model":"model-from-payload","output_format":"payload","findings":[],"attack_log":[{"target":"diff","attack":"scan","result":"clean"}],"budget":{"actual_attack_angles":1}}}` + "\n"
	dossier, err := (CommandProvider{
		Command: "reviewer",
		Runner:  &fakeRunner{result: execution.Result{ExitCode: 0, Stdout: text}},
	}).Invoke(context.Background(), review.Request{TaskID: "task", Prompt: "prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if dossier.Provider != "command" || dossier.OutputFormat != "command.stdout" {
		t.Fatalf("transport provenance was not authoritative: %+v", dossier)
	}
	if dossier.Model != "model-from-payload" {
		t.Fatalf("command provider may preserve self-reported model, got %+v", dossier)
	}
}

func TestCommandProviderFailsClosedOnMissingRunnerInvalidOutputAndCleanPacketNonzeroExit(t *testing.T) {
	t.Parallel()

	if _, err := (CommandProvider{Command: "reviewer"}).Invoke(context.Background(), review.Request{TaskID: "task"}); !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("missing runner err = %v", err)
	}
	if _, err := (CommandProvider{Command: "reviewer", Runner: &fakeRunner{result: execution.Result{Stdout: "{invalid\n"}}}).Invoke(context.Background(), review.Request{TaskID: "task"}); !errors.Is(err, review.ErrInvalidDossier) {
		t.Fatalf("invalid output err = %v", err)
	}
	runner := &fakeRunner{result: execution.Result{ExitCode: 1, Stdout: dossierFrame(review.VerdictPass) + "\n"}}
	if _, err := (CommandProvider{Command: "reviewer", Runner: runner}).Invoke(context.Background(), review.Request{TaskID: "task"}); !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("nonzero clean packet err = %v", err)
	}
}

func TestCommandProviderFailureIncludesDiagnosticPath(t *testing.T) {
	t.Parallel()

	diagnostic := filepath.Join(t.TempDir(), "diagnostic.txt")
	_, err := (CommandProvider{Command: "reviewer", Runner: &fakeRunner{result: execution.Result{
		Stdout:         "{invalid\n",
		DiagnosticPath: diagnostic,
	}}}).Invoke(context.Background(), review.Request{TaskID: "task"})
	if !errors.Is(err, ErrProviderFailed) || !errors.Is(err, review.ErrInvalidDossier) {
		t.Fatalf("error = %v, want provider failure wrapping invalid dossier", err)
	}
	if !strings.Contains(err.Error(), diagnostic) {
		t.Fatalf("error missing diagnostic path %q: %v", diagnostic, err)
	}
}

func TestHardenCommandProviderFailureIncludesDiagnostics(t *testing.T) {
	t.Parallel()

	diagnostic := filepath.Join(t.TempDir(), "harden-diagnostic.txt")
	_, err := (HardenProvider{
		Agent: CommandProvider{
			Command: "harden-reviewer",
			Runner: &fakeRunner{result: execution.Result{
				Stdout:         "{invalid\n",
				DiagnosticPath: diagnostic,
			}},
		},
	}).Invoke(context.Background(), coreharden.Request{TaskID: "task"})
	if !errors.Is(err, ErrProviderFailed) || !errors.Is(err, coreharden.ErrInvalidDossier) {
		t.Fatalf("error = %v, want provider failure wrapping invalid harden dossier", err)
	}
	if !strings.Contains(err.Error(), diagnostic) {
		t.Fatalf("error missing diagnostic path %q: %v", diagnostic, err)
	}
}

func TestClaudeProviderBuildsRestrictedStreamJSONArgsAndExtractsStructuredOutput(t *testing.T) {
	t.Parallel()

	stdout := `{"type":"system","subtype":"init","model":"claude-test","session_id":"observed-session"}` + "\n" +
		`{"type":"result","result":"submitted"}` + "\n"
	runner := &fakeRunner{
		result: execution.Result{Stdout: stdout},
		onRun: func(req execution.Request) {
			writeClaudeSubmission(t, req, dossierJSON(review.VerdictFail))
		},
	}
	dossier, err := (ClaudeProvider{
		Binary:       "claude-bin",
		Model:        "claude-model",
		SessionID:    "00000000-0000-4000-8000-000000000000",
		ScafldBinary: "scafld-bin",
		CWD:          "/tmp/work",
		Runner:       runner,
	}).Invoke(context.Background(), review.Request{TaskID: "task", Prompt: "prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if dossier.Verdict != review.VerdictFail || len(dossier.Findings) != 1 {
		t.Fatalf("dossier = %+v", dossier)
	}
	if dossier.Provider != "claude" || dossier.Model != "claude-test" || dossier.SessionID == "" || dossier.OutputFormat != "claude.mcp_submit_review" || dossier.EventSummary["system.init"] != 1 || dossier.EventSummary["result"] != 1 {
		t.Fatalf("provenance = %+v", dossier)
	}
	wantPrefix := []string{
		"claude-bin", "-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--no-session-persistence", "--disable-slash-commands", "--no-chrome",
		"--permission-mode", "dontAsk", "--setting-sources", "user", "--tools", "Read,Grep,Glob",
		"--allowedTools", "Read,Grep,Glob,mcp__scafld__submit_review",
		"--disallowedTools", "Agent,Task,Bash,Edit,MultiEdit,Write,NotebookEdit",
		"--mcp-config",
	}
	if len(runner.req.Args) < len(wantPrefix) || !reflect.DeepEqual(runner.req.Args[:len(wantPrefix)], wantPrefix) || runner.req.Input != "prompt" || runner.req.CWD != "/tmp/work" {
		t.Fatalf("request = %+v", runner.req)
	}
	command, _ := claudeSubmissionCommand(t, runner.req)
	if command != "scafld-bin" {
		t.Fatalf("mcp command = %q", command)
	}
}

func TestClaudeProviderRequiresMCPSubmissionAndIgnoresResultText(t *testing.T) {
	t.Parallel()

	stdout := `{"type":"system","subtype":"init","model":"claude-test","session_id":"observed-session"}` + "\n" +
		`{"type":"result","result":"Will produce the requested dossier.\n` + "```json" + `\n` + strings.ReplaceAll(dossierJSON(review.VerdictPass), `"`, `\"`) + `\n` + "```" + `"}` + "\n"
	runner := &fakeRunner{result: execution.Result{Stdout: stdout}}
	dossier, err := (ClaudeProvider{Runner: runner, SessionID: "00000000-0000-4000-8000-000000000000"}).Invoke(context.Background(), review.Request{TaskID: "task"})
	if !errors.Is(err, ErrProviderFailed) || !strings.Contains(err.Error(), "provider produced no submission") || !strings.Contains(err.Error(), "submit_review") {
		t.Fatalf("err = %v dossier=%+v", err, dossier)
	}
}

func TestClaudeProviderRejectsInvalidSubmittedDossier(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		result: execution.Result{Stdout: `{"type":"result","result":"submitted"}` + "\n"},
		onRun: func(req execution.Request) {
			writeClaudeSubmission(t, req, dossierWithStringLocationJSON())
		},
	}
	_, err := (ClaudeProvider{Runner: runner, SessionID: "00000000-0000-4000-8000-000000000000"}).Invoke(context.Background(), review.Request{TaskID: "task"})
	if !errors.Is(err, review.ErrInvalidDossier) {
		t.Fatalf("err = %v", err)
	}
}

func TestCodexProviderBuildsReadOnlyEphemeralArgsAndReadsOutputFile(t *testing.T) {
	t.Parallel()

	outputPath := t.TempDir() + "/dossier.json"
	runner := &fakeRunner{
		result: execution.Result{Stdout: "progress only"},
		onRun: func(execution.Request) {
			if err := os.WriteFile(outputPath, []byte(dossierJSON(review.VerdictPass)), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	dossier, err := (CodexProvider{
		Binary:     "codex-bin",
		Model:      "gpt-test",
		SchemaPath: "/tmp/schema.json",
		OutputPath: outputPath,
		CWD:        "/tmp/work",
		Runner:     runner,
	}).Invoke(context.Background(), review.Request{TaskID: "task", Prompt: "prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if dossier.Verdict != review.VerdictPass {
		t.Fatalf("dossier = %+v", dossier)
	}
	if dossier.Provider != "codex" || dossier.OutputFormat != "codex.output_file" {
		t.Fatalf("dossier = %+v", dossier)
	}
	wantArgs := []string{
		"codex-bin", "exec", "--sandbox", "read-only", "--skip-git-repo-check", "--cd", "/tmp/work",
		"--ephemeral", "--ignore-user-config", "--ignore-rules", "--color", "never", "--output-last-message", outputPath,
		"--output-schema", "/tmp/schema.json", "-m", "gpt-test",
	}
	if !reflect.DeepEqual(runner.req.Args, wantArgs) || runner.req.Input != "prompt" {
		t.Fatalf("request = %+v", runner.req)
	}
}

func TestCodexProviderWritesDefaultSchema(t *testing.T) {
	t.Parallel()

	var schemaPath string
	outputPath := t.TempDir() + "/dossier.json"
	runner := &fakeRunner{
		result: execution.Result{Stdout: ""},
		onRun: func(req execution.Request) {
			for i, arg := range req.Args {
				if arg == "--output-schema" && i+1 < len(req.Args) {
					schemaPath = req.Args[i+1]
				}
			}
			if schemaPath == "" {
				t.Fatal("missing output schema")
			}
			if data, err := os.ReadFile(schemaPath); err != nil || !strings.Contains(string(data), `"verdict"`) {
				t.Fatalf("schema data = %q err=%v", data, err)
			}
			if err := os.WriteFile(outputPath, []byte(dossierJSON(review.VerdictPass)), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	_, err := (CodexProvider{OutputPath: outputPath, Runner: runner}).Invoke(context.Background(), review.Request{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGeminiProviderBuildsPlanModeMCPArgsAndRequiresSubmission(t *testing.T) {
	t.Parallel()

	settingsPath := filepath.Join(t.TempDir(), "gemini-settings.json")
	policyPath := filepath.Join(t.TempDir(), "gemini-policy.toml")
	runner := &fakeRunner{
		result: execution.Result{Stdout: `{"type":"message","model":"gemini-test"}` + "\n"},
		onRun: func(req execution.Request) {
			writeGeminiSubmission(t, req, dossierJSON(review.VerdictPass))
		},
	}
	dossier, err := (GeminiProvider{
		Binary:       "gemini-bin",
		Model:        "gemini-model",
		ScafldBinary: "scafld-bin",
		SettingsPath: settingsPath,
		PolicyPath:   policyPath,
		CWD:          "/tmp/work",
		Runner:       runner,
	}).Invoke(context.Background(), review.Request{TaskID: "task", Prompt: "prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if dossier.Verdict != review.VerdictPass || dossier.Provider != "gemini" || dossier.Model != "gemini-test" || dossier.OutputFormat != "gemini.mcp_submit_review" {
		t.Fatalf("dossier = %+v", dossier)
	}
	wantPrefix := []string{
		"gemini-bin", "--skip-trust", "--approval-mode", "plan", "--output-format", "stream-json",
		"--allowed-mcp-server-names", "scafld", "--policy", policyPath,
		"--prompt", "",
	}
	if len(runner.req.Args) < len(wantPrefix) || !reflect.DeepEqual(runner.req.Args[:len(wantPrefix)], wantPrefix) || runner.req.CWD != "/tmp/work" {
		t.Fatalf("request = %+v", runner.req)
	}
	if !strings.Contains(runner.req.Input, "mcp_scafld_submit_review") {
		t.Fatalf("prompt missing Gemini submit tool name: %q", runner.req.Input)
	}
	settings := geminiSettingsPath(t, runner.req)
	data, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"defaultApprovalMode":"plan"`) || !strings.Contains(string(data), `"includeTools":["submit_review"]`) || !strings.Contains(string(data), `"command":"scafld-bin"`) {
		t.Fatalf("settings = %s", data)
	}
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(policy), `mcpName = "scafld"`) || !strings.Contains(string(policy), `toolName = "submit_review"`) || !strings.Contains(string(policy), `decision = "allow"`) {
		t.Fatalf("policy = %s", policy)
	}
}

func TestGeminiProviderFailsWhenToolSubmissionIsMissing(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: execution.Result{Stdout: `{"type":"message","text":"done"}` + "\n"}}
	_, err := (GeminiProvider{Runner: runner}).Invoke(context.Background(), review.Request{TaskID: "task"})
	if !errors.Is(err, ErrProviderFailed) || !strings.Contains(err.Error(), "provider produced no submission") || !strings.Contains(err.Error(), "mcp_scafld_submit_review") {
		t.Fatalf("err = %v", err)
	}
}

func TestReviewDossierSchemaIsStrictStructuredOutputCompatible(t *testing.T) {
	t.Parallel()

	var root map[string]any
	if err := json.Unmarshal([]byte(ReviewDossierSchemaJSON()), &root); err != nil {
		t.Fatal(err)
	}
	assertStrictStructuredOutputSchema(t, "$", root)
}

func TestReviewDossierSchemaMatchesManagedCoreAsset(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate provider test file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	asset, err := os.ReadFile(filepath.Join(root, "internal", "adapters", "corebundle", "assets", "core", "schemas", "review_dossier.json"))
	if err != nil {
		t.Fatal(err)
	}
	if normalizeJSON(t, string(asset)) != normalizeJSON(t, review.DossierSchemaJSON()) {
		t.Fatal("bundled review_dossier.json drifted from core schema generator")
	}
}

func TestHardenDossierSchemaMatchesManagedCoreAsset(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate provider test file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	asset, err := os.ReadFile(filepath.Join(root, "internal", "adapters", "corebundle", "assets", "core", "schemas", "harden_dossier.json"))
	if err != nil {
		t.Fatal(err)
	}
	if normalizeJSON(t, string(asset)) != normalizeJSON(t, coreharden.DossierSchemaJSON()) {
		t.Fatal("bundled harden_dossier.json drifted from core schema generator")
	}
}

func TestProviderFailureSurfacesCapturedStderrWhenOutputIsEmpty(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: execution.Result{
		ExitCode: 1,
		Stderr:   `ERROR: {"code":"invalid_json_schema","message":"Missing 'line'."}`,
	}}
	_, err := (CodexProvider{OutputPath: t.TempDir() + "/empty.json", SchemaPath: "/tmp/schema.json", Runner: runner}).Invoke(context.Background(), review.Request{TaskID: "task"})
	if !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("err = %v, want provider failure", err)
	}
	if !strings.Contains(err.Error(), "invalid_json_schema") || strings.Contains(err.Error(), "empty provider output") {
		t.Fatalf("provider error should surface stderr, got: %v", err)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func assertStrictStructuredOutputSchema(t *testing.T, path string, node map[string]any) {
	t.Helper()
	if properties, ok := node["properties"].(map[string]any); ok {
		if !schemaAllowsObject(node["type"]) {
			t.Fatalf("%s has properties but does not allow object: %#v", path, node["type"])
		}
		if node["additionalProperties"] != false {
			t.Fatalf("%s must set additionalProperties false", path)
		}
		required := stringSet(node["required"])
		for key := range properties {
			if !required[key] {
				t.Fatalf("%s required does not include property %q", path, key)
			}
		}
		for key, raw := range properties {
			child, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			assertStrictStructuredOutputSchema(t, path+"."+key, child)
		}
	}
	if rawItems, ok := node["items"].(map[string]any); ok {
		assertStrictStructuredOutputSchema(t, path+"[]", rawItems)
	}
	if additional, ok := node["additionalProperties"].(map[string]any); ok {
		t.Fatalf("%s uses dynamic additionalProperties schema, which provider structured output does not accept: %#v", path, additional)
	}
}

func schemaAllowsObject(raw any) bool {
	switch value := raw.(type) {
	case string:
		return value == "object"
	case []any:
		for _, item := range value {
			if item == "object" {
				return true
			}
		}
	}
	return false
}

func stringSet(raw any) map[string]bool {
	set := map[string]bool{}
	items, ok := raw.([]any)
	if !ok {
		return set
	}
	for _, item := range items {
		if key, ok := item.(string); ok {
			set[key] = true
		}
	}
	return set
}

func normalizeJSON(t *testing.T, text string) string {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestClaudeEventName(t *testing.T) {
	t.Parallel()

	if got := ClaudeEventName(`{"type":"result","subtype":"success"}`); got != "result.success" {
		t.Fatalf("event = %q", got)
	}
	if got := ClaudeEventName(`not-json`); got != "" {
		t.Fatalf("event = %q", got)
	}
}
