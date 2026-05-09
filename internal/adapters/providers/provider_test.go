package providers

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
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

func dossierJSON(verdict string) string {
	if verdict == review.VerdictFail {
		return `{"verdict":"fail","mode":"discover","summary":"bug found","findings":[{"id":"bug","severity":"high","blocks_completion":true,"location":{"path":"file.go"},"evidence":"bug","impact":"breaks behavior","validation":"rerun tests","summary":"bug"}],"attack_log":[{"target":"diff","attack":"scan","result":"finding"}],"budget":{"actual_findings":1,"actual_attack_angles":1,"depth":"test"}}`
	}
	return `{"verdict":"pass","mode":"discover","summary":"clean","findings":[],"attack_log":[{"target":"diff","attack":"scan","result":"clean"}],"budget":{"actual_attack_angles":1,"depth":"test"}}`
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

func TestClaudeProviderBuildsRestrictedStreamJSONArgsAndExtractsStructuredOutput(t *testing.T) {
	t.Parallel()

	stdout := `{"type":"system","subtype":"init","model":"claude-test","session_id":"observed-session"}` + "\n" +
		`{"type":"result","structured_output":` + dossierJSON(review.VerdictFail) + `}` + "\n"
	runner := &fakeRunner{result: execution.Result{Stdout: stdout}}
	dossier, err := (ClaudeProvider{
		Binary:     "claude-bin",
		Model:      "claude-model",
		SessionID:  "00000000-0000-4000-8000-000000000000",
		SchemaJSON: `{"type":"object"}`,
		CWD:        "/tmp/work",
		Runner:     runner,
	}).Invoke(context.Background(), review.Request{TaskID: "task", Prompt: "prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if dossier.Verdict != review.VerdictFail || len(dossier.Findings) != 1 {
		t.Fatalf("dossier = %+v", dossier)
	}
	if dossier.Provider != "claude" || dossier.Model != "claude-test" || dossier.SessionID == "" || dossier.EventSummary["system.init"] != 1 || dossier.EventSummary["result"] != 1 {
		t.Fatalf("provenance = %+v", dossier)
	}
	wantArgs := []string{
		"claude-bin", "-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--permission-mode", "plan", "--allowedTools", "Read,Grep,Glob",
		"--disallowedTools", "Agent,Task,Bash,Edit,MultiEdit,Write,NotebookEdit",
		"--mcp-config", `{"mcpServers":{}}`, "--strict-mcp-config",
		"--session-id", "00000000-0000-4000-8000-000000000000",
		"--json-schema", `{"type":"object"}`, "--model", "claude-model",
	}
	if !reflect.DeepEqual(runner.req.Args, wantArgs) || runner.req.Input != "prompt" || runner.req.CWD != "/tmp/work" {
		t.Fatalf("request = %+v", runner.req)
	}
}

func TestClaudeProviderAttachesDefaultSchema(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: execution.Result{Stdout: `{"type":"result","structured_output":` + dossierJSON(review.VerdictPass) + `}` + "\n"}}
	_, err := (ClaudeProvider{Runner: runner, SessionID: "00000000-0000-4000-8000-000000000000"}).Invoke(context.Background(), review.Request{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsArg(runner.req.Args, "--json-schema") {
		t.Fatalf("args missing --json-schema: %+v", runner.req.Args)
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
	if dossier.Provider != "codex" {
		t.Fatalf("provider = %q", dossier.Provider)
	}
	wantArgs := []string{
		"codex-bin", "exec", "--sandbox", "read-only", "--skip-git-repo-check", "--cd", "/tmp/work",
		"--ephemeral", "--ignore-user-config", "--color", "never", "--output-last-message", outputPath,
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

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
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
