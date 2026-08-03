package providers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
	"github.com/nilstate/scafld/v2/internal/core/providerpacket"
	"github.com/nilstate/scafld/v2/internal/core/review"
)

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
	dossier, dossierErr := hardenDossierFromProviderResult(resp)
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
	dossier, dossierErr := dossierFromProviderResult(resp, parse)
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

func dossierFromProviderResult(resp AgentResponse, parse func(string) (review.Dossier, error)) (review.Dossier, error) {
	result, runErr, text := resp.Result, resp.RunErr, resp.Text
	if runErr != nil && strings.TrimSpace(text) == "" {
		return review.Dossier{}, providerFailedError(result, runErr)
	}
	dossier, parseErr := parse(text)
	if parseErr != nil {
		if runErr != nil {
			return review.Dossier{}, providerFailedError(result, runErr)
		}
		if result.DiagnosticPath != "" {
			return review.Dossier{}, providerFailedPacketError(result, parseErr, packetSourceFromResponse(resp, "ReviewDossier", "submit_review", parseErr))
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

func hardenDossierFromProviderResult(resp AgentResponse) (coreharden.Dossier, error) {
	result, runErr, text := resp.Result, resp.RunErr, resp.Text
	if runErr != nil && strings.TrimSpace(text) == "" {
		return coreharden.Dossier{}, providerFailedError(result, runErr)
	}
	dossier, parseErr := coreharden.ParseText(text)
	if parseErr != nil {
		if runErr != nil {
			return coreharden.Dossier{}, providerFailedError(result, runErr)
		}
		if result.DiagnosticPath != "" {
			return coreharden.Dossier{}, providerFailedPacketError(result, parseErr, packetSourceFromResponse(resp, "HardenDossier", "submit_harden", parseErr))
		}
		if result.ExitCode != 0 {
			return coreharden.Dossier{}, providerFailedError(result, fmt.Errorf("exit code %d", result.ExitCode))
		}
		return coreharden.Dossier{}, parseErr
	}
	if runErr != nil {
		return coreharden.Dossier{}, providerFailedError(result, runErr)
	}
	if result.ExitCode != 0 && coreharden.VerdictFromDossier(dossier) != coreharden.VerdictNeedsRevision {
		return coreharden.Dossier{}, providerFailedError(result, fmt.Errorf("exit code %d", result.ExitCode))
	}
	return dossier, nil
}

func providerFailedError(result execution.Result, cause error) error {
	return providerFailedPacketError(result, cause, providerpacket.Source{})
}

func providerFailedPacketError(result execution.Result, cause error, source providerpacket.Source) error {
	detail := ""
	if stderr := errorSnippet(result.Stderr); stderr != "" {
		detail += ": " + stderr
	} else if stdout := errorSnippet(result.Stdout); stdout != "" {
		detail += ": " + stdout
	}
	if result.DiagnosticPath != "" {
		detail += " (diagnostic: " + result.DiagnosticPath + ")"
	}
	if strings.TrimSpace(source.DiagnosticPath) == "" {
		source.DiagnosticPath = result.DiagnosticPath
	}
	if strings.TrimSpace(source.Error) == "" && cause != nil {
		source.Error = cause.Error()
	}
	return providerFailureError{cause: cause, detail: detail, diagnosticPath: result.DiagnosticPath, packetSource: source}
}

type providerFailureError struct {
	cause          error
	detail         string
	diagnosticPath string
	packetSource   providerpacket.Source
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

func (e providerFailureError) ProviderPacketRepairSource() providerpacket.Source {
	return e.packetSource
}

func packetSourceFromResponse(resp AgentResponse, schemaName string, submitTool string, cause error) providerpacket.Source {
	return providerpacket.Source{
		Provider:           resp.Provider,
		Model:              resp.Model,
		OutputFormat:       resp.OutputFormat,
		ExpectedSchema:     schemaName,
		ExpectedSubmitTool: submitTool,
		Error:              errorString(cause),
		DiagnosticPath:     resp.Result.DiagnosticPath,
		RejectedText:       strings.TrimSpace(resp.Text),
	}
}

func providerpacketSource(provider string, model string, outputFormat string, schemaName string, submitTool string, rejectedText string, cause error) providerpacket.Source {
	return providerpacket.Source{
		Provider:           provider,
		Model:              model,
		OutputFormat:       outputFormat,
		ExpectedSchema:     schemaName,
		ExpectedSubmitTool: submitTool,
		Error:              errorString(cause),
		RejectedText:       strings.TrimSpace(rejectedText),
	}
}

func providerNoSubmissionSource(provider string, model string, outputFormat string, schemaName string, submitTool string, cause error, candidates ...string) providerpacket.Source {
	return providerpacket.Source{
		Provider:           provider,
		Model:              model,
		OutputFormat:       outputFormat,
		ExpectedSchema:     schemaName,
		ExpectedSubmitTool: submitTool,
		Error:              errorString(cause),
		RejectedText:       noSubmissionRepairText(schemaName, candidates...),
	}
}

func noSubmissionRepairText(schemaName string, candidates ...string) string {
	for _, candidate := range packetTextCandidates(candidates...) {
		if providerPacketParses(schemaName, candidate) || providerPacketShape(schemaName, candidate) {
			return candidate
		}
	}
	return ""
}

func providerPacketParses(schemaName string, text string) bool {
	switch schemaName {
	case "HardenDossier":
		_, err := coreharden.ParseText(text)
		return err == nil
	case "ReviewDossier":
		_, err := review.ParseText(text)
		return err == nil
	default:
		return false
	}
}

func providerPacketShape(schemaName string, text string) bool {
	fields, ok := jsonObjectFields(text)
	if !ok {
		return false
	}
	switch schemaName {
	case "HardenDossier":
		return knownFieldCount(fields, "summary", "shape", "observations", "provider", "model", "output_format") >= 2
	case "ReviewDossier":
		if rawType, hasType := fields["type"]; hasType {
			var frameType string
			if err := json.Unmarshal(rawType, &frameType); err == nil && frameType == "dossier" {
				_, hasDossier := fields["dossier"]
				return hasDossier
			}
		}
		return knownFieldCount(fields, "verdict", "mode", "summary", "findings", "attack_log", "budget", "provider", "model", "output_format") >= 2
	default:
		return false
	}
}

func knownFieldCount(fields map[string]json.RawMessage, names ...string) int {
	count := 0
	for _, name := range names {
		if _, ok := fields[name]; ok {
			count++
		}
	}
	return count
}

func jsonObjectFields(text string) (map[string]json.RawMessage, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "{") {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &fields); err != nil {
		return nil, false
	}
	return fields, true
}

func packetTextCandidates(candidates ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if !seen[candidate] {
			seen[candidate] = true
			out = append(out, candidate)
		}
		for _, fenced := range fencedJSONCandidates(candidate) {
			if !seen[fenced] {
				seen[fenced] = true
				out = append(out, fenced)
			}
		}
	}
	return out
}

func fencedJSONCandidates(text string) []string {
	var out []string
	rest := text
	for {
		start := strings.Index(rest, "```")
		if start < 0 {
			return out
		}
		rest = rest[start+3:]
		end := strings.Index(rest, "```")
		if end < 0 {
			return out
		}
		block := strings.TrimSpace(rest[:end])
		rest = rest[end+3:]
		if first, body, ok := strings.Cut(block, "\n"); ok {
			if language := strings.ToLower(strings.TrimSpace(first)); language == "json" || language == "jsonc" {
				block = strings.TrimSpace(body)
			}
		}
		if block != "" {
			out = append(out, block)
		}
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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

func claudeResultTexts(stdout string) []string {
	var out []string
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event["type"] == "result" {
			if result := stringField(event, "result"); result != "" {
				out = append(out, result)
			}
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
		Description: "Submit the final scafld ReviewDossier. Call exactly once after completing the read-only adversarial review. Findings are defects only; improvements, preferences, marginal-surface enumeration, and per-consumer bookkeeping are not findings unless they identify a verified defect, violated shared invariant, or broken adapter boundary. Clean attack_log entries must name the concrete target inspected.",
		Command:     "review-submit-stdio",
	}
}

func hardenSubmitTool() SubmitTool {
	return SubmitTool{
		Name:        "submit_harden",
		Title:       "Submit scafld hardening",
		Description: "Submit the final scafld HardenDossier. Call exactly once after stress-testing the draft spec. Treat the draft as a hypothesis: if reject/no-op, shrink, reframe, move-owner, or reuse-existing-behavior is materially better, record that in shape instead of softening it into advisory feedback. Harden is a code-shape and system-design gate, not coverage bookkeeping; do not turn a settled shared invariant into a consumer-by-consumer compliance matrix or bespoke test-per-surface demand. The shape object must answer keep, shrink, reframe, or reject before observations. The observations array must cover " + coreharden.RequiredDimensionList() + " with result, anchor, and any note/question/recommended/if_unanswered/default/status. A keep decision and clean observations must name what was checked and what would have failed the check.",
		Command:     "harden-submit-stdio",
	}
}

func localHardenDossier() string {
	return `{"summary":"Local provider smoke hardening passed.","shape":{"decision":"keep","true_shape":"Local smoke hardening keeps the draft shape for transport validation.","minimal_plan":"Submit a schema-valid harden dossier without changing files.","shared_owner":"internal/core/harden","adapter_boundaries":["local provider emits the dossier","harden app derives the verdict"],"required_spec_edits":[]},"observations":[{"dimension":"design","result":"clean","anchor":"spec_gap:Summary","note":"Local smoke provider records required harden observations."},{"dimension":"scope","result":"clean","anchor":"spec_gap:Scope","note":"Local smoke provider records required harden observations."},{"dimension":"path","result":"clean","anchor":"spec_gap:Context","note":"Local smoke provider records required harden observations."},{"dimension":"command","result":"clean","anchor":"spec_gap:Acceptance","note":"Local smoke provider records required harden observations."},{"dimension":"timing","result":"clean","anchor":"spec_gap:Acceptance","note":"Local smoke provider records required harden observations."},{"dimension":"rollback","result":"clean","anchor":"spec_gap:Rollback","note":"Local smoke provider records required harden observations."}]}`
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
