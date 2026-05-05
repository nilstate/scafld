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
	packet, err := (LocalProvider{Messages: []string{`{"type":"finding","severity":"blocking","summary":"bug"}`}}).Invoke(context.Background(), review.Request{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if packet.Verdict != "fail" || packet.Provider != "local" || len(packet.Findings) != 1 {
		t.Fatalf("packet = %+v", packet)
	}
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
		Stdout:   `{"type":"finding","severity":"blocking","summary":"bug"}` + "\n",
		Stderr:   "progress\n",
		Output:   "progress should not be parsed",
	}}
	packet, err := (CommandProvider{
		Command:     "reviewer",
		CWD:         "/tmp/work",
		Runner:      runner,
		Timeout:     time.Minute,
		IdleTimeout: time.Second,
	}).Invoke(context.Background(), review.Request{TaskID: "task", Prompt: "prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if packet.Verdict != review.VerdictFail || packet.Provider != "command" || len(packet.Findings) != 1 {
		t.Fatalf("packet = %+v", packet)
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
	if _, err := (CommandProvider{Command: "reviewer", Runner: &fakeRunner{result: execution.Result{Stdout: "{invalid\n"}}}).Invoke(context.Background(), review.Request{TaskID: "task"}); !errors.Is(err, review.ErrInvalidPacket) {
		t.Fatalf("invalid output err = %v", err)
	}
	runner := &fakeRunner{result: execution.Result{ExitCode: 1, Stdout: `{"type":"verdict","verdict":"pass"}` + "\n"}}
	if _, err := (CommandProvider{Command: "reviewer", Runner: runner}).Invoke(context.Background(), review.Request{TaskID: "task"}); !errors.Is(err, ErrProviderFailed) {
		t.Fatalf("nonzero clean packet err = %v", err)
	}
}

func TestClaudeProviderBuildsRestrictedStreamJSONArgsAndExtractsStructuredOutput(t *testing.T) {
	t.Parallel()

	stdout := `{"type":"system","subtype":"init","model":"claude-test","session_id":"observed-session"}` + "\n" +
		`{"type":"result","structured_output":{"findings":[{"severity":"blocking","summary":"bug"}]}}` + "\n"
	runner := &fakeRunner{result: execution.Result{Stdout: stdout}}
	packet, err := (ClaudeProvider{
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
	if packet.Verdict != review.VerdictFail || len(packet.Findings) != 1 {
		t.Fatalf("packet = %+v", packet)
	}
	if packet.Provider != "claude" || packet.Model != "claude-test" || packet.SessionID == "" || packet.EventSummary["system.init"] != 1 || packet.EventSummary["result"] != 1 {
		t.Fatalf("provenance = %+v", packet)
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

	runner := &fakeRunner{result: execution.Result{Stdout: `{"type":"result","structured_output":{"verdict":"pass"}}` + "\n"}}
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

	outputPath := t.TempDir() + "/packet.json"
	runner := &fakeRunner{
		result: execution.Result{Stdout: "progress only"},
		onRun: func(execution.Request) {
			if err := os.WriteFile(outputPath, []byte(`{"verdict":"pass"}`), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	packet, err := (CodexProvider{
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
	if packet.Verdict != review.VerdictPass {
		t.Fatalf("packet = %+v", packet)
	}
	if packet.Provider != "codex" {
		t.Fatalf("provider = %q", packet.Provider)
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
	outputPath := t.TempDir() + "/packet.json"
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
			if err := os.WriteFile(outputPath, []byte(`{"verdict":"pass"}`), 0o644); err != nil {
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
