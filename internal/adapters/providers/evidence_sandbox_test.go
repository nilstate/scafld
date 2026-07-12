package providers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/execution"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
)

func TestEvidenceFileSandboxNormalizesAndExposesProvenance(t *testing.T) {
	t.Parallel()

	file := evidenceFile("dir/../src\\file.go", reviewevidence.StatusModified, "package src\n")
	sandbox, err := BuildEvidenceSandbox([]reviewevidence.EvidenceFile{file})
	if err != nil {
		t.Fatal(err)
	}
	defer sandbox.Cleanup()

	if sandbox.CWD == "" || !reflect.DeepEqual(sandbox.ReadRoots, []string{sandbox.CWD}) {
		t.Fatalf("sandbox roots = cwd %q roots %v", sandbox.CWD, sandbox.ReadRoots)
	}
	if !sandbox.ArgsPolicy.MemoryAutoloadDisabled || !sandbox.Policy.MemoryAutoloadDisabled {
		t.Fatalf("sandbox memory policy missing: %+v %+v", sandbox.ArgsPolicy, sandbox.Policy)
	}
	if len(sandbox.Provenance) != 1 {
		t.Fatalf("provenance = %+v", sandbox.Provenance)
	}
	provenance := sandbox.Provenance[0]
	if provenance.Path != "src/file.go" || provenance.Status != reviewevidence.StatusModified || provenance.SHA256 != file.SHA256 {
		t.Fatalf("provenance = %+v", provenance)
	}
	data, err := os.ReadFile(provenance.ScratchPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "package src\n" || !strings.HasPrefix(provenance.ScratchPath, sandbox.CWD+string(filepath.Separator)) {
		t.Fatalf("scratch materialization wrong: path=%q data=%q", provenance.ScratchPath, data)
	}
}

func TestEvidenceFileHashMismatchFailsBeforeWriting(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	_, err := BuildEvidenceSandbox([]reviewevidence.EvidenceFile{{
		Path:   "src/file.go",
		Status: reviewevidence.StatusModified,
		Bytes:  []byte("package src\n"),
		SHA256: strings.Repeat("0", 64),
	}})
	if err == nil || !strings.Contains(err.Error(), "hash mismatch") {
		t.Fatalf("err = %v, want hash mismatch", err)
	}
	entries, readErr := os.ReadDir(tmp)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("hash mismatch wrote scratch files: %v", entries)
	}
}

func TestEvidenceSandboxPathInsideRejectsTraversal(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "sandbox")
	if pathInside(root, filepath.Join(root, "nested", "file.go")) != true {
		t.Fatal("path inside sandbox should be accepted")
	}
	if pathInside(root, filepath.Join(root, "..", "outside.go")) {
		t.Fatal("path traversal outside sandbox should be rejected")
	}
	if pathInside(root, root) {
		t.Fatal("sandbox root itself is not a materialized evidence file")
	}
}

func TestEvidenceFileBlocklistRejectsAgentInstructionFiles(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"CLAUDE.md", "docs/AGENTS.md", ".scafld/config.yaml", "nested/GEMINI.md"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			_, err := BuildEvidenceSandbox([]reviewevidence.EvidenceFile{evidenceFile(path, reviewevidence.StatusModified, "do not read me\n")})
			if err == nil || !strings.Contains(err.Error(), "evidence") {
				t.Fatalf("err = %v, want blocklist rejection for %s", err, path)
			}
		})
	}

	sandbox, err := BuildEvidenceSandbox([]reviewevidence.EvidenceFile{evidenceFile("docs/NOT_AGENTS.md", reviewevidence.StatusModified, "review me\n")})
	if err != nil {
		t.Fatal(err)
	}
	sandbox.Cleanup()
}

func TestReceiptGradeCodexSandboxReadRootMemory(t *testing.T) {
	t.Parallel()

	agent, facts := selectReceiptGradeSandboxAgent(t, "codex")
	wrapped := agent.(receiptGradeSandboxAgent)
	codex, ok := wrapped.Agent.(CodexProvider)
	if !ok {
		t.Fatalf("agent = %T, want CodexProvider", wrapped.Agent)
	}
	assertReceiptGradeSandboxProvider(t, codex.CWD, codex.ReadRoots, codex.MemoryAutoloadDisabled, codex.SandboxPolicy, facts)
	args := CodexArgs(codex.Binary, codex.CWD, "/tmp/out.json", codex.Model, "")
	for _, want := range []string{"--sandbox", "read-only", "--ignore-user-config", "--ignore-rules", "--cd", codex.CWD} {
		if !containsArg(args, want) {
			t.Fatalf("codex args missing %q: %v", want, args)
		}
	}
}

func TestReceiptGradeClaudeSandboxMemoryReadRoot(t *testing.T) {
	t.Parallel()

	agent, facts := selectReceiptGradeSandboxAgent(t, "claude")
	wrapped := agent.(receiptGradeSandboxAgent)
	claude, ok := wrapped.Agent.(ClaudeProvider)
	if !ok {
		t.Fatalf("agent = %T, want ClaudeProvider", wrapped.Agent)
	}
	assertReceiptGradeSandboxProvider(t, claude.CWD, claude.ReadRoots, claude.MemoryAutoloadDisabled, claude.SandboxPolicy, facts)
	args := ClaudeArgs(claude.Binary, claude.Model, "00000000-0000-4000-8000-000000000000", "{}", SubmitTool{Name: "submit_review"}, claude.ReadRoots)
	for _, want := range []string{"--no-session-persistence", "--disable-slash-commands", "--permission-mode", "dontAsk", "--setting-sources", "user", "--disallowedTools", "Agent,Task,Bash,Edit,MultiEdit,Write,NotebookEdit", "--add-dir", claude.CWD} {
		if !containsArg(args, want) {
			t.Fatalf("claude args missing %q: %v", want, args)
		}
	}
}

func TestReceiptGradeGeminiSandboxSettingsMemoryReadRoot(t *testing.T) {
	t.Parallel()

	agent, facts := selectReceiptGradeSandboxAgent(t, "gemini")
	wrapped := agent.(receiptGradeSandboxAgent)
	gemini, ok := wrapped.Agent.(GeminiProvider)
	if !ok {
		t.Fatalf("agent = %T, want GeminiProvider", wrapped.Agent)
	}
	assertReceiptGradeSandboxProvider(t, gemini.CWD, gemini.ReadRoots, gemini.MemoryAutoloadDisabled, gemini.SandboxPolicy, facts)
	settings := GeminiSettingsJSON("scafld-bin", "/tmp/out.json", SubmitTool{Name: "submit_review"}, gemini.SandboxPolicy)
	var decoded struct {
		Context struct {
			IncludeDirectories []string `json:"includeDirectories"`
		} `json:"context"`
	}
	if err := json.Unmarshal([]byte(settings), &decoded); err != nil {
		t.Fatalf("decode Gemini settings: %v", err)
	}
	if !reflect.DeepEqual(decoded.Context.IncludeDirectories, []string{gemini.CWD}) {
		t.Fatalf("gemini settings missing read-scope: %s", settings)
	}
}

func TestReceiptGradeReviewReturnsRuntimeFactsProvenance(t *testing.T) {
	t.Parallel()

	binary := writeExecutable(t, "codex-reviewer")
	var request execution.Request
	runner := &fakeRunner{
		result: execution.Result{ExitCode: 0, Stdout: ""},
		onRun: func(req execution.Request) {
			request = req
			outputPath := argAfter(req.Args, "--output-last-message")
			if outputPath == "" {
				t.Fatalf("missing codex output path: %v", req.Args)
			}
			if err := os.WriteFile(outputPath, []byte(`{"ok":true}`), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	}
	result, err := InvokeReceiptGradeReview(context.Background(), ReceiptGradeReviewInput{
		Selection: Selection{
			Provider:          "codex",
			CodexBinary:       binary,
			CodexEndpointHost: "api.openai.com",
			Runner:            runner,
		},
		HostEnviron: []string{"OPENAI_API_KEY=secret"},
		Evidence:    []reviewevidence.EvidenceFile{evidenceFile("src/file.go", reviewevidence.StatusModified, "package src\n")},
		Request:     AgentRequest{TaskID: "task", Prompt: "prompt", SchemaName: "ReviewDossier"},
	})
	if err != nil {
		t.Fatal(err)
	}
	facts := result.RuntimeFacts
	if facts.BinarySHA256 == "" || facts.EndpointHost != "api.openai.com" || len(facts.EvidenceProvenance) != 1 || len(facts.SandboxPolicy.ReadRoots) != 1 {
		t.Fatalf("runtime facts = %+v", facts)
	}
	if result.Response.BinarySHA256 != facts.BinarySHA256 || result.Response.EndpointHost != facts.EndpointHost || len(result.Response.EvidenceProvenance) != 1 || len(result.Response.SandboxPolicy.ReadRoots) != 1 {
		t.Fatalf("response facts = %+v runtime=%+v", result.Response, facts)
	}
	if request.CWD != facts.SandboxPolicy.ReadRoots[0] || request.EnvMode != execution.EnvModeExact {
		t.Fatalf("request did not use sandbox exact env: %+v facts=%+v", request, facts)
	}
	if _, err := os.Stat(facts.SandboxPolicy.ReadRoots[0]); !os.IsNotExist(err) {
		t.Fatalf("sandbox cleanup did not remove %s: %v", facts.SandboxPolicy.ReadRoots[0], err)
	}
}

func TestReceiptGradeEmptyEvidenceStillSandboxed(t *testing.T) {
	t.Parallel()

	binary := writeExecutable(t, "codex-reviewer")
	selection := Selection{
		Provider:          "codex",
		CodexBinary:       binary,
		CodexEndpointHost: "api.openai.com",
		Runner:            &fakeRunner{},
		CommandExists:     func(string) bool { return false },
	}
	// A deletion-only or all-withheld scope yields zero byte-bearing evidence; the
	// reviewer must still run in an isolated sandbox, never from the repo root with
	// host HOME/config reachable.
	agent, facts, err := SelectReceiptGradeAgentWithEvidence(selection, []string{"OPENAI_API_KEY=secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, ok := agent.(receiptGradeSandboxAgent)
	if !ok {
		t.Fatalf("empty-evidence review must still be sandboxed, got %T", agent)
	}
	t.Cleanup(func() {
		if wrapped.sandbox.Cleanup != nil {
			wrapped.sandbox.Cleanup()
		}
	})
	if len(facts.SandboxPolicy.ReadRoots) != 1 || !facts.SandboxPolicy.MemoryAutoloadDisabled {
		t.Fatalf("empty-evidence sandbox policy not applied: %+v", facts.SandboxPolicy)
	}
	codex, ok := wrapped.Agent.(CodexProvider)
	if !ok {
		t.Fatalf("agent = %T, want CodexProvider", wrapped.Agent)
	}
	if codex.CWD == "" || codex.CWD != facts.SandboxPolicy.ReadRoots[0] {
		t.Fatalf("reviewer CWD must be the isolated sandbox root: cwd=%q roots=%v", codex.CWD, facts.SandboxPolicy.ReadRoots)
	}
	if len(facts.EvidenceProvenance) != 0 {
		t.Fatalf("empty evidence must carry no provenance: %+v", facts.EvidenceProvenance)
	}
}

func TestReceiptGradeReadRootEnforcementRecordedHonestly(t *testing.T) {
	t.Parallel()

	// Codex's read-only sandbox does not jail reads, so the receipt must record its
	// read confinement as best-effort, not enforced.
	_, codexFacts := selectReceiptGradeSandboxAgent(t, "codex")
	if codexFacts.SandboxPolicy.ReadRootsEnforced {
		t.Fatal("codex read confinement is best-effort; the receipt must not claim read roots are enforced")
	}
	// Claude hard-confines reads via --add-dir, so enforcement is recorded as true.
	_, claudeFacts := selectReceiptGradeSandboxAgent(t, "claude")
	if !claudeFacts.SandboxPolicy.ReadRootsEnforced {
		t.Fatal("claude --add-dir hard-confines reads; the receipt must record read roots as enforced")
	}
}

func selectReceiptGradeSandboxAgent(t *testing.T, provider string) (Agent, RuntimeFacts) {
	t.Helper()
	binary := writeExecutable(t, provider+"-reviewer")
	selection := Selection{
		Provider:      provider,
		Runner:        &fakeRunner{},
		CommandExists: func(string) bool { return false },
	}
	env := []string{}
	switch provider {
	case "claude":
		selection.ClaudeBinary = binary
		selection.ClaudeEndpointHost = "api.anthropic.com"
		env = []string{"ANTHROPIC_API_KEY=secret"}
	case "gemini":
		selection.GeminiBinary = binary
		selection.GeminiEndpointHost = "generativelanguage.googleapis.com"
		env = []string{"GEMINI_API_KEY=secret"}
	default:
		selection.CodexBinary = binary
		selection.CodexEndpointHost = "api.openai.com"
		env = []string{"OPENAI_API_KEY=secret"}
	}
	agent, facts, err := SelectReceiptGradeAgentWithEvidence(selection, env, []reviewevidence.EvidenceFile{
		evidenceFile("src/file.go", reviewevidence.StatusModified, "package src\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if wrapped, ok := agent.(receiptGradeSandboxAgent); ok && wrapped.sandbox.Cleanup != nil {
			wrapped.sandbox.Cleanup()
		}
	})
	return agent, facts
}

func assertReceiptGradeSandboxProvider(t *testing.T, cwd string, readRoots []string, memoryOff bool, policy SandboxPolicy, facts RuntimeFacts) {
	t.Helper()
	if cwd == "" || !reflect.DeepEqual(readRoots, []string{cwd}) || !memoryOff {
		t.Fatalf("provider sandbox = cwd %q roots %v memoryOff %v", cwd, readRoots, memoryOff)
	}
	if !reflect.DeepEqual(policy.ReadRoots, []string{cwd}) || !policy.MemoryAutoloadDisabled {
		t.Fatalf("provider policy = %+v", policy)
	}
	if !reflect.DeepEqual(facts.SandboxPolicy.ReadRoots, []string{cwd}) || !facts.SandboxPolicy.MemoryAutoloadDisabled || len(facts.EvidenceProvenance) != 1 {
		t.Fatalf("facts = %+v", facts)
	}
}

func evidenceFile(path string, status string, text string) reviewevidence.EvidenceFile {
	bytes := []byte(text)
	return reviewevidence.EvidenceFile{
		Path:   path,
		Status: status,
		Bytes:  bytes,
		SHA256: reviewevidence.SHA256Hex(bytes),
	}
}

func writeExecutable(t *testing.T, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func argAfter(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
