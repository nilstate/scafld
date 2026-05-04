package providers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
	"github.com/nilstate/scafld/v2/internal/core/review"
)

var ErrProviderFailed = errors.New("provider failed")

type Runner interface {
	Run(context.Context, execution.Request) (execution.Result, error)
}

type LocalProvider struct {
	Messages []string
}

func (p LocalProvider) Invoke(ctx context.Context, req review.Request) (review.Packet, error) {
	var lines []string
	for _, msg := range p.Messages {
		if err := ctx.Err(); err != nil {
			return review.Packet{}, err
		}
		lines = append(lines, msg)
	}
	if len(lines) == 0 {
		lines = []string{`{"type":"verdict","verdict":"pass"}`}
	}
	return review.ParseNDJSON(strings.Join(lines, "\n") + "\n")
}

type CommandProvider struct {
	Command     string
	CWD         string
	Env         []string
	Runner      Runner
	Timeout     time.Duration
	IdleTimeout time.Duration
}

func (p CommandProvider) Invoke(ctx context.Context, req review.Request) (review.Packet, error) {
	if p.Runner == nil {
		return review.Packet{}, fmt.Errorf("%w: runner is required", ErrProviderFailed)
	}
	if strings.TrimSpace(p.Command) == "" {
		return review.Packet{}, fmt.Errorf("%w: command is required", ErrProviderFailed)
	}
	env := append([]string(nil), p.Env...)
	env = append(env, "SCAFLD_TASK_ID="+req.TaskID)
	result, err := p.Runner.Run(ctx, execution.Request{
		Command:     p.Command,
		Input:       req.Prompt,
		CWD:         p.CWD,
		Env:         env,
		Timeout:     p.Timeout,
		IdleTimeout: p.IdleTimeout,
	})
	if err != nil && strings.TrimSpace(result.Stdout) == "" {
		return review.Packet{}, fmt.Errorf("%w: %v", ErrProviderFailed, err)
	}
	packet, parseErr := review.ParseText(result.Stdout)
	if parseErr != nil {
		if err != nil {
			return review.Packet{}, fmt.Errorf("%w: %v", ErrProviderFailed, err)
		}
		return review.Packet{}, parseErr
	}
	if err != nil {
		return review.Packet{}, fmt.Errorf("%w: %v", ErrProviderFailed, err)
	}
	if result.ExitCode != 0 && packet.Verdict != review.VerdictFail {
		return review.Packet{}, fmt.Errorf("%w: exit code %d", ErrProviderFailed, result.ExitCode)
	}
	return packet, nil
}

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

func (p ClaudeProvider) Invoke(ctx context.Context, req review.Request) (review.Packet, error) {
	if p.Runner == nil {
		return review.Packet{}, fmt.Errorf("%w: runner is required", ErrProviderFailed)
	}
	sessionID := p.SessionID
	if sessionID == "" {
		sessionID = newUUID()
	}
	schemaJSON := p.SchemaJSON
	if schemaJSON == "" {
		schemaJSON = ReviewPacketSchemaJSON()
	}
	result, err := p.Runner.Run(ctx, execution.Request{
		Args:                 ClaudeArgs(binaryOrDefault(p.Binary, "claude"), p.Model, sessionID, schemaJSON),
		Input:                req.Prompt,
		CWD:                  p.CWD,
		Env:                  p.Env,
		Timeout:              p.Timeout,
		IdleTimeout:          p.IdleTimeout,
		StdoutEventInspector: ClaudeEventName,
	})
	extracted := extractClaudeOutput(result.Stdout)
	packet, packetErr := packetFromProviderResult(result, err, extracted.Body)
	if packetErr != nil {
		return review.Packet{}, packetErr
	}
	packet.Provider = "claude"
	packet.Model = extracted.Model
	packet.SessionID = extracted.SessionID
	packet.EventSummary = eventSummary(result.StdoutEvents, extracted.Events)
	return packet, nil
}

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

func (p CodexProvider) Invoke(ctx context.Context, req review.Request) (review.Packet, error) {
	if p.Runner == nil {
		return review.Packet{}, fmt.Errorf("%w: runner is required", ErrProviderFailed)
	}
	outputPath := p.OutputPath
	cleanup := func() {}
	if outputPath == "" {
		file, err := os.CreateTemp("", "scafld-codex-review-*.json")
		if err != nil {
			return review.Packet{}, fmt.Errorf("%w: create output file: %v", ErrProviderFailed, err)
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
			return review.Packet{}, fmt.Errorf("%w: create schema file: %v", ErrProviderFailed, err)
		}
		schemaPath = file.Name()
		if _, err := file.WriteString(ReviewPacketSchemaJSON()); err != nil {
			_ = file.Close()
			return review.Packet{}, fmt.Errorf("%w: write schema file: %v", ErrProviderFailed, err)
		}
		_ = file.Close()
		cleanupSchema = func() { _ = os.Remove(schemaPath) }
	}
	defer cleanupSchema()
	result, err := p.Runner.Run(ctx, execution.Request{
		Args:        CodexArgs(binaryOrDefault(p.Binary, "codex"), p.CWD, outputPath, p.Model, schemaPath),
		Input:       req.Prompt,
		CWD:         p.CWD,
		Env:         p.Env,
		Timeout:     p.Timeout,
		IdleTimeout: p.IdleTimeout,
	})
	body := strings.TrimSpace(result.Stdout)
	if data, readErr := os.ReadFile(filepath.Clean(outputPath)); readErr == nil && strings.TrimSpace(string(data)) != "" {
		body = string(data)
	}
	packet, packetErr := packetFromProviderResult(result, err, body)
	if packetErr != nil {
		return review.Packet{}, packetErr
	}
	packet.Provider = "codex"
	return packet, nil
}

func ClaudeArgs(binary string, model string, sessionID string, schemaJSON string) []string {
	args := []string{
		binary,
		"-p",
		"--output-format",
		"stream-json",
		"--verbose",
		"--include-partial-messages",
		"--permission-mode",
		"plan",
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

func ReviewPacketSchemaJSON() string {
	return `{"type":"object","additionalProperties":false,"required":["verdict","findings"],"properties":{"verdict":{"type":"string","enum":["pass","fail"]},"findings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["id","severity","summary"],"properties":{"id":{"type":"string"},"severity":{"type":"string","enum":["blocking","non_blocking"]},"summary":{"type":"string"}}}}}}`
}

func packetFromProviderResult(result execution.Result, runErr error, text string) (review.Packet, error) {
	if runErr != nil && strings.TrimSpace(text) == "" {
		return review.Packet{}, fmt.Errorf("%w: %v", ErrProviderFailed, runErr)
	}
	packet, parseErr := review.ParseText(text)
	if parseErr != nil {
		if runErr != nil {
			return review.Packet{}, fmt.Errorf("%w: %v", ErrProviderFailed, runErr)
		}
		return review.Packet{}, parseErr
	}
	if runErr != nil {
		return review.Packet{}, fmt.Errorf("%w: %v", ErrProviderFailed, runErr)
	}
	if result.ExitCode != 0 && packet.Verdict != review.VerdictFail {
		return review.Packet{}, fmt.Errorf("%w: exit code %d", ErrProviderFailed, result.ExitCode)
	}
	return packet, nil
}

type claudeOutput struct {
	Body      string
	Model     string
	SessionID string
	Events    map[string]int
}

func extractClaudeOutput(stdout string) claudeOutput {
	out := claudeOutput{Body: stdout, Events: map[string]int{}}
	var result map[string]any
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
		if event["type"] == "result" {
			result = event
		}
	}
	if len(result) > 0 {
		if structured, ok := result["structured_output"].(map[string]any); ok {
			if data, err := json.Marshal(structured); err == nil {
				out.Body = string(data)
				return out
			}
		}
		for _, key := range []string{"result", "output", "response", "text", "content"} {
			if value, ok := result[key].(string); ok && strings.TrimSpace(value) != "" {
				out.Body = value
				return out
			}
		}
	}
	return out
}

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
