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
	"github.com/nilstate/scafld/v2/internal/core/review"
)

// ErrProviderFailed wraps provider transport and execution failures.
var ErrProviderFailed = errors.New("provider failed")

// Runner is the process execution port required by external providers.
type Runner interface {
	Run(context.Context, execution.Request) (execution.Result, error)
}

// Selection contains provider choice, model, timeout, and runner configuration.
type Selection struct {
	Provider       string
	Command        string
	Binary         string
	Model          string
	CodexModel     string
	ClaudeModel    string
	CodexBinary    string
	ClaudeBinary   string
	CWD            string
	Runner         Runner
	Timeout        time.Duration
	Idle           time.Duration
	FallbackPolicy string
}

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
	default:
		return nil, fmt.Errorf("unknown review provider %q", opts.Provider)
	}
}

func selectAuto(opts Selection) (interface {
	Invoke(context.Context, review.Request) (review.Dossier, error)
}, error) {
	if opts.Binary != "" {
		return CodexProvider{Binary: opts.Binary, Model: opts.Model, CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	}
	if _, err := osexec.LookPath("codex"); err == nil {
		return CodexProvider{Binary: opts.CodexBinary, Model: first(opts.Model, opts.CodexModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	}
	if opts.FallbackPolicy == "disable" {
		return nil, errors.New("codex review provider not found and review.external.fallback_policy is disable")
	}
	if _, err := osexec.LookPath("claude"); err == nil {
		return ClaudeProvider{Binary: opts.ClaudeBinary, Model: first(opts.Model, opts.ClaudeModel), CWD: opts.CWD, Runner: opts.Runner, Timeout: opts.Timeout, IdleTimeout: opts.Idle}, nil
	}
	return nil, errors.New("no external review provider found; install codex or claude, use --provider command --provider-command <cmd>, or use --provider local for development smoke tests")
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

// Invoke returns a dossier from configured local messages.
func (p LocalProvider) Invoke(ctx context.Context, req review.Request) (review.Dossier, error) {
	var lines []string
	for _, msg := range p.Messages {
		if err := ctx.Err(); err != nil {
			return review.Dossier{}, err
		}
		lines = append(lines, msg)
	}
	if len(lines) == 0 {
		lines = []string{`{"type":"dossier","dossier":{"verdict":"pass","mode":"discover","summary":"Local provider smoke review passed.","findings":[],"attack_log":[{"target":"local provider","attack":"deterministic smoke review","result":"clean"}],"budget":{"actual_attack_angles":1,"depth":"local"}}}`}
	}
	dossier, err := review.ParseNDJSON(strings.Join(lines, "\n") + "\n")
	dossier.Provider = "local"
	return dossier, err
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

// Invoke sends the review prompt to the command and parses stdout as a dossier.
func (p CommandProvider) Invoke(ctx context.Context, req review.Request) (review.Dossier, error) {
	if p.Runner == nil {
		return review.Dossier{}, fmt.Errorf("%w: runner is required", ErrProviderFailed)
	}
	if strings.TrimSpace(p.Command) == "" {
		return review.Dossier{}, fmt.Errorf("%w: command is required", ErrProviderFailed)
	}
	env := append([]string(nil), p.Env...)
	env = append(env, "SCAFLD_TASK_ID="+req.TaskID)
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
		return review.Dossier{}, fmt.Errorf("%w: %v", ErrProviderFailed, err)
	}
	dossier, parseErr := review.ParseText(result.Stdout)
	if parseErr != nil {
		if err != nil {
			return review.Dossier{}, fmt.Errorf("%w: %v", ErrProviderFailed, err)
		}
		return review.Dossier{}, parseErr
	}
	if err != nil {
		return review.Dossier{}, fmt.Errorf("%w: %v", ErrProviderFailed, err)
	}
	if result.ExitCode != 0 && dossier.Verdict != review.VerdictFail {
		return review.Dossier{}, fmt.Errorf("%w: exit code %d", ErrProviderFailed, result.ExitCode)
	}
	dossier.Provider = "command"
	dossier.OutputFormat = first(dossier.OutputFormat, "command.stdout")
	return dossier, nil
}

// ClaudeProvider invokes Claude in restricted stream-json review mode.
type ClaudeProvider struct {
	Binary      string
	Model       string
	SessionID   string
	SchemaJSON  string
	CWD         string
	Env         []string
	Runner      Runner
	Timeout     time.Duration
	IdleTimeout time.Duration
}

// Invoke sends the review prompt to Claude and parses the resulting dossier.
func (p ClaudeProvider) Invoke(ctx context.Context, req review.Request) (review.Dossier, error) {
	if p.Runner == nil {
		return review.Dossier{}, fmt.Errorf("%w: runner is required", ErrProviderFailed)
	}
	sessionID := p.SessionID
	if sessionID == "" {
		sessionID = newUUID()
	}
	schemaJSON := p.SchemaJSON
	if schemaJSON == "" {
		schemaJSON = ReviewDossierSchemaJSON()
	}
	result, err := p.Runner.Run(ctx, execution.Request{
		Args:                   ClaudeArgs(binaryOrDefault(p.Binary, "claude"), p.Model, sessionID, schemaJSON),
		Input:                  req.Prompt,
		CWD:                    p.CWD,
		Env:                    p.Env,
		Timeout:                p.Timeout,
		IdleTimeout:            p.IdleTimeout,
		SuppressProgressStderr: true,
		StdoutEventInspector:   ClaudeEventName,
	})
	extracted := extractClaudeOutput(result.Stdout)
	dossier, dossierErr := dossierFromProviderResult(result, err, extracted.Body)
	if dossierErr != nil {
		return review.Dossier{}, dossierErr
	}
	dossier.Provider = "claude"
	dossier.Model = extracted.Model
	dossier.SessionID = extracted.SessionID
	dossier.OutputFormat = extracted.Format
	dossier.EventSummary = eventSummary(result.StdoutEvents, extracted.Events)
	return dossier, nil
}

// CodexProvider invokes Codex in read-only ephemeral review mode.
type CodexProvider struct {
	Binary      string
	Model       string
	SchemaPath  string
	OutputPath  string
	CWD         string
	Env         []string
	Runner      Runner
	Timeout     time.Duration
	IdleTimeout time.Duration
}

// Invoke sends the review prompt to Codex and parses the resulting dossier.
func (p CodexProvider) Invoke(ctx context.Context, req review.Request) (review.Dossier, error) {
	if p.Runner == nil {
		return review.Dossier{}, fmt.Errorf("%w: runner is required", ErrProviderFailed)
	}
	outputPath := p.OutputPath
	cleanup := func() {}
	if outputPath == "" {
		file, err := os.CreateTemp("", "scafld-codex-review-*.json")
		if err != nil {
			return review.Dossier{}, fmt.Errorf("%w: create output file: %v", ErrProviderFailed, err)
		}
		outputPath = file.Name()
		_ = file.Close()
		cleanup = func() { _ = os.Remove(outputPath) }
	}
	defer cleanup()
	schemaPath := p.SchemaPath
	cleanupSchema := func() {}
	if schemaPath == "" {
		file, err := os.CreateTemp("", "scafld-review-schema-*.json")
		if err != nil {
			return review.Dossier{}, fmt.Errorf("%w: create schema file: %v", ErrProviderFailed, err)
		}
		schemaPath = file.Name()
		if _, err := file.WriteString(ReviewDossierSchemaJSON()); err != nil {
			_ = file.Close()
			return review.Dossier{}, fmt.Errorf("%w: write schema file: %v", ErrProviderFailed, err)
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
	dossier, dossierErr := dossierFromProviderResult(result, err, body)
	if dossierErr != nil {
		return review.Dossier{}, dossierErr
	}
	dossier.Provider = "codex"
	dossier.OutputFormat = first(dossier.OutputFormat, outputFormat)
	return dossier, nil
}

// ClaudeArgs builds the argv for restricted Claude review execution.
func ClaudeArgs(binary string, model string, sessionID string, schemaJSON string) []string {
	args := []string{
		binary,
		"-p",
		"--output-format",
		"stream-json",
		"--verbose",
		"--include-partial-messages",
		"--allowedTools",
		"Read,Grep,Glob",
		"--disallowedTools",
		"Agent,Task,Bash,Edit,MultiEdit,Write,NotebookEdit",
		"--mcp-config",
		`{"mcpServers":{}}`,
		"--strict-mcp-config",
		"--session-id",
		sessionID,
	}
	if schemaJSON != "" {
		args = append(args, "--json-schema", schemaJSON)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	return args
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

func dossierFromProviderResult(result execution.Result, runErr error, text string) (review.Dossier, error) {
	if runErr != nil && strings.TrimSpace(text) == "" {
		return review.Dossier{}, providerFailedError(result, runErr)
	}
	dossier, parseErr := review.ParseText(text)
	if parseErr != nil {
		if runErr != nil {
			return review.Dossier{}, providerFailedError(result, runErr)
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

func providerFailedError(result execution.Result, cause error) error {
	message := strings.TrimSpace(cause.Error())
	if stderr := errorSnippet(result.Stderr); stderr != "" {
		message += ": " + stderr
	} else if stdout := errorSnippet(result.Stdout); stdout != "" {
		message += ": " + stdout
	}
	if result.DiagnosticPath != "" {
		message += " (diagnostic: " + result.DiagnosticPath + ")"
	}
	return fmt.Errorf("%w: %s", ErrProviderFailed, message)
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

type claudeOutput struct {
	Body      string
	Format    string
	Model     string
	SessionID string
	Events    map[string]int
}

func extractClaudeOutput(stdout string) claudeOutput {
	out := claudeOutput{Body: stdout, Format: "claude.stdout", Events: map[string]int{}}
	var result map[string]any
	sawStreamEvent := false
	bodyFromResultText := false
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event["type"] != nil {
			sawStreamEvent = true
		}
		if name := ClaudeEventName(line); name != "" {
			out.Events[name]++
		}
		if event["type"] == "system" && event["subtype"] == "init" {
			out.Model = stringField(event, "model", "model_id", "modelId")
			out.SessionID = stringField(event, "session_id", "sessionId")
		}
		if event["type"] == "result" {
			result = event
		}
	}
	if len(result) > 0 {
		if structured, ok := result["structured_output"].(map[string]any); ok {
			if data, err := json.Marshal(structured); err == nil {
				out.Body = string(data)
				out.Format = "claude.structured_output"
				return out
			}
		}
		for _, key := range []string{"result", "output", "response", "text", "content"} {
			if value, ok := result[key].(string); ok && strings.TrimSpace(value) != "" {
				out.Body = value
				out.Format = "claude.result_text"
				bodyFromResultText = true
				break
			}
		}
	}
	if bodyFromResultText || !sawStreamEvent {
		if extracted, format, ok := extractJSONEnvelope(out.Body); ok {
			out.Body = extracted
			if bodyFromResultText {
				out.Format = "claude.result_text." + format
			} else {
				out.Format = "claude.stdout." + format
			}
		}
	}
	return out
}

func extractJSONEnvelope(text string) (string, string, bool) {
	trimmed := strings.TrimSpace(text)
	if json.Valid([]byte(trimmed)) {
		return trimmed, "json", true
	}
	if candidate, ok := extractFencedJSON(text); ok {
		return candidate, "fenced_json", true
	}
	if candidate, ok := extractBalancedJSONObject(text); ok {
		return candidate, "balanced_json", true
	}
	return "", "", false
}

func extractFencedJSON(text string) (string, bool) {
	for _, lang := range []string{"json", "JSON", ""} {
		fence := "```" + lang
		searchFrom := 0
		for {
			start := strings.Index(text[searchFrom:], fence)
			if start < 0 {
				break
			}
			start += searchFrom + len(fence)
			if lang != "" && start < len(text) {
				switch text[start] {
				case '\r':
					start++
					if start < len(text) && text[start] == '\n' {
						start++
					}
				case '\n':
					start++
				}
			}
			end := strings.Index(text[start:], "```")
			if end < 0 {
				break
			}
			candidate := strings.TrimSpace(text[start : start+end])
			if json.Valid([]byte(candidate)) {
				return candidate, true
			}
			searchFrom = start + end + len("```")
		}
	}
	return "", false
}

func extractBalancedJSONObject(text string) (string, bool) {
	start := strings.Index(text, "{")
	for start >= 0 {
		inString := false
		escaped := false
		depth := 0
		for i := start; i < len(text); i++ {
			ch := text[i]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				switch ch {
				case '\\':
					escaped = true
				case '"':
					inString = false
				}
				continue
			}
			switch ch {
			case '"':
				inString = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					candidate := strings.TrimSpace(text[start : i+1])
					if json.Valid([]byte(candidate)) {
						return candidate, true
					}
					next := strings.Index(text[start+1:], "{")
					if next < 0 {
						return "", false
					}
					start += next + 1
					goto nextCandidate
				}
			}
		}
		return "", false
	nextCandidate:
	}
	return "", false
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

func binaryOrDefault(binary string, fallback string) string {
	if strings.TrimSpace(binary) == "" {
		return fallback
	}
	return binary
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
