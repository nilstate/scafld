package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
	"github.com/nilstate/scafld/v2/internal/core/review"
)

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
	Effort                 string
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
		Args:                   ClaudeArgs(binaryOrDefault(p.Binary, "claude"), p.Model, p.Effort, sessionID, ClaudeMCPConfig(scafldBinaryOrDefault(p.ScafldBinary), submissionPath, tool), tool, p.ReadRoots),
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
	ModelReasoningEffort   string
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
		Args:                   CodexArgs(binaryOrDefault(p.Binary, "codex"), p.CWD, outputPath, p.Model, p.ModelReasoningEffort, schemaPath),
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
	return AgentResponse{Text: body, Provider: "codex", Model: p.Model, OutputFormat: outputFormat, BinarySHA256: p.BinarySHA256, EndpointHost: p.EndpointHost, SandboxPolicy: p.SandboxPolicy, Result: result, RunErr: err}, nil
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
func ClaudeArgs(binary string, model string, effort string, sessionID string, mcpConfig string, tool SubmitTool, readRoots []string) []string {
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
		"--permission-mode",
		"dontAsk",
		"--setting-sources",
		"user",
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
	if effort != "" {
		args = append(args, "--effort", effort)
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
func CodexArgs(binary string, root string, outputPath string, model string, modelReasoningEffort string, schemaPath string) []string {
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
	if modelReasoningEffort != "" {
		args = append(args, "-c", "model_reasoning_effort="+strconv.Quote(modelReasoningEffort))
	}
	if model != "" {
		args = append(args, "-m", model)
	}
	return args
}
