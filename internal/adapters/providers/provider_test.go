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
	if !errors.Is(err, ErrProviderFailed) || !strings.Contains(err.Error(), "provider produced no submission") {
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
	asset, err := os.ReadFile(filepath.Join(root, ".scafld", "core", "schemas", "review_dossier.json"))
	if err != nil {
		t.Fatal(err)
	}
	if normalizeJSON(t, string(asset)) != normalizeJSON(t, review.DossierSchemaJSON()) {
		t.Fatal("managed review_dossier.json drifted from core schema generator")
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
