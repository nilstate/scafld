package review

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
)

// Severity names defect impact independent from whether the finding blocks
// task completion.
type Severity string

const (
	// SeverityCritical marks correctness, security, data-loss, or release-stopping impact.
	SeverityCritical Severity = "critical"
	// SeverityHigh marks substantial product, architecture, or reliability impact.
	SeverityHigh Severity = "high"
	// SeverityMedium marks meaningful but bounded impact.
	SeverityMedium Severity = "medium"
	// SeverityLow marks polish, clarity, or minor maintainability impact.
	SeverityLow Severity = "low"
)

// Confidence describes how strongly the reviewer believes a finding is real.
type Confidence string

const (
	// ConfidenceHigh means the finding is directly supported by cited evidence.
	ConfidenceHigh Confidence = "high"
	// ConfidenceMedium means the finding is plausible but may need validation.
	ConfidenceMedium Confidence = "medium"
	// ConfidenceLow means the finding is speculative and must not block completion.
	ConfidenceLow Confidence = "low"
)

// FindingStatus tracks whether a previously surfaced finding still needs work.
type FindingStatus string

const (
	// FindingOpen means the finding still needs repair.
	FindingOpen FindingStatus = "open"
	// FindingFixed means the reviewer verified the finding as repaired.
	FindingFixed FindingStatus = "fixed"
	// FindingAcceptedRisk means a human accepted the residual risk.
	FindingAcceptedRisk FindingStatus = "accepted_risk"
	// FindingSuperseded means a newer finding replaced this one.
	FindingSuperseded FindingStatus = "superseded"
)

// Mode describes the review pass shape.
type Mode string

const (
	// ModeDiscover searches broadly for new findings.
	ModeDiscover Mode = "discover"
	// ModeVerify checks whether known findings are fixed and whether repairs introduced regressions.
	ModeVerify Mode = "verify"
)

// AttackResult names the bounded result of one review attack.
type AttackResult string

const (
	// AttackResultFinding means the attack produced one or more findings.
	AttackResultFinding AttackResult = "finding"
	// AttackResultClean means the attack found no issue.
	AttackResultClean AttackResult = "clean"
	// AttackResultSkipped means the reviewer deliberately skipped the attack and should explain why.
	AttackResultSkipped AttackResult = "skipped"
)

const (
	// VerdictPass marks a dossier with no open completion blockers.
	VerdictPass = "pass"
	// VerdictFail marks a dossier with one or more open completion blockers.
	VerdictFail = "fail"
)

// ErrInvalidDossier wraps malformed or semantically invalid provider output.
var ErrInvalidDossier = errors.New("invalid review dossier")

// Location points to the primary evidence location for a finding.
type Location struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
}

// Finding is one issue surfaced by a review provider.
type Finding struct {
	ID               string        `json:"id"`
	Severity         Severity      `json:"severity"`
	BlocksCompletion bool          `json:"blocks_completion"`
	Category         string        `json:"category,omitempty"`
	Confidence       Confidence    `json:"confidence,omitempty"`
	Location         *Location     `json:"location,omitempty"`
	Evidence         string        `json:"evidence,omitempty"`
	Impact           string        `json:"impact,omitempty"`
	Reproducer       string        `json:"reproducer,omitempty"`
	SuggestedFix     string        `json:"suggested_fix,omitempty"`
	Validation       string        `json:"validation,omitempty"`
	RelatedSpec      string        `json:"related_spec,omitempty"`
	ReviewPass       string        `json:"review_pass,omitempty"`
	Status           FindingStatus `json:"status,omitempty"`
	Summary          string        `json:"summary,omitempty"`
}

// AttackLogEntry records one bounded attack angle the reviewer attempted.
type AttackLogEntry struct {
	Target string       `json:"target"`
	Attack string       `json:"attack"`
	Result AttackResult `json:"result"`
	Notes  string       `json:"notes,omitempty"`
}

// Budget records review depth and bounded output settings.
type Budget struct {
	MaxFindings        int    `json:"max_findings,omitempty"`
	MinAttackAngles    int    `json:"min_attack_angles,omitempty"`
	ActualFindings     int    `json:"actual_findings,omitempty"`
	ActualAttackAngles int    `json:"actual_attack_angles,omitempty"`
	Depth              string `json:"depth,omitempty"`
}

// Dossier is the normalized review-provider payload consumed by scafld.
type Dossier struct {
	Verdict        string           `json:"verdict"`
	Mode           Mode             `json:"mode"`
	Summary        string           `json:"summary"`
	Findings       []Finding        `json:"findings"`
	AttackLog      []AttackLogEntry `json:"attack_log"`
	Budget         Budget           `json:"budget"`
	Provider       string           `json:"provider,omitempty"`
	Model          string           `json:"model,omitempty"`
	SessionID      string           `json:"session_id,omitempty"`
	OutputFormat   string           `json:"output_format,omitempty"`
	Normalizations []string         `json:"normalizations,omitempty"`
	EventSummary   map[string]int   `json:"event_summary,omitempty"`
	Raw            string           `json:"-"`
}

// Request is the provider-facing review prompt request.
type Request struct {
	TaskID  string
	Prompt  string
	Context reviewcontext.Packet
}

// EncodeDossier serializes a dossier for session storage.
func EncodeDossier(dossier Dossier) string {
	if strings.TrimSpace(dossier.Verdict) == "" && len(dossier.Findings) == 0 && strings.TrimSpace(dossier.Summary) == "" {
		return ""
	}
	data, err := json.Marshal(dossier)
	if err != nil {
		return ""
	}
	return string(data)
}

// DecodeDossier parses a dossier recorded in a session review entry.
func DecodeDossier(text string) (Dossier, bool) {
	if strings.TrimSpace(text) == "" {
		return Dossier{}, false
	}
	var dossier Dossier
	if err := json.Unmarshal([]byte(text), &dossier); err != nil {
		return Dossier{}, false
	}
	dossier = NormalizeDossier(dossier)
	if err := ValidateDossier(dossier); err != nil {
		return Dossier{}, false
	}
	return dossier, true
}

// OpenBlockerCount returns the number of findings that currently block completion.
func OpenBlockerCount(findings []Finding) int {
	count := 0
	for _, finding := range findings {
		if BlocksCompletion(finding) {
			count++
		}
	}
	return count
}

// BlocksCompletion reports whether a finding still blocks completion.
func BlocksCompletion(finding Finding) bool {
	if !finding.BlocksCompletion {
		return false
	}
	switch finding.Status {
	case "", FindingOpen:
		return true
	case FindingFixed, FindingAcceptedRisk, FindingSuperseded:
		return false
	default:
		return true
	}
}

// ValidCompletionProvider reports whether a passing review provider can satisfy
// the completion gate for real work.
func ValidCompletionProvider(provider string) bool {
	switch strings.TrimSpace(provider) {
	case "codex", "claude", "command", "human":
		return true
	default:
		return false
	}
}

// ParseText parses direct JSON dossiers or NDJSON review streams.
func ParseText(text string) (Dossier, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Dossier{}, fmt.Errorf("%w: empty provider output", ErrInvalidDossier)
	}
	if strings.HasPrefix(trimmed, "{") {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
			return Dossier{}, fmt.Errorf("%w: %v", ErrInvalidDossier, err)
		}
		if _, hasType := probe["type"]; !hasType {
			var dossier Dossier
			if err := json.Unmarshal([]byte(trimmed), &dossier); err != nil {
				return Dossier{}, fmt.Errorf("%w: %v", ErrInvalidDossier, err)
			}
			dossier.Raw = text
			dossier = NormalizeDossier(dossier)
			if err := ValidateDossier(dossier); err != nil {
				return Dossier{}, err
			}
			return dossier, nil
		}
	}
	return ParseNDJSON(text)
}

// ParseNDJSON parses newline-delimited review events into a dossier.
func ParseNDJSON(text string) (Dossier, error) {
	var dossier Dossier
	found := false
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var frame struct {
			Type    string          `json:"type"`
			Dossier json.RawMessage `json:"dossier"`
		}
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			return Dossier{}, fmt.Errorf("%w: %v", ErrInvalidDossier, err)
		}
		switch frame.Type {
		case "dossier":
			raw := frame.Dossier
			if len(raw) == 0 {
				raw = []byte(line)
			}
			if err := json.Unmarshal(raw, &dossier); err != nil {
				return Dossier{}, fmt.Errorf("%w: %v", ErrInvalidDossier, err)
			}
			found = true
		case "tick", "partial":
		case "verdict", "finding", "workspace_mutation", "done":
			return Dossier{}, fmt.Errorf("%w: legacy review frame %q is no longer supported; emit one review dossier", ErrInvalidDossier, frame.Type)
		default:
			return Dossier{}, fmt.Errorf("%w: unknown frame type %q", ErrInvalidDossier, frame.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return Dossier{}, fmt.Errorf("%w: %v", ErrInvalidDossier, err)
	}
	if !found {
		return Dossier{}, fmt.Errorf("%w: missing dossier frame", ErrInvalidDossier)
	}
	dossier.Raw = text
	dossier = NormalizeDossier(dossier)
	if err := ValidateDossier(dossier); err != nil {
		return Dossier{}, err
	}
	return dossier, nil
}

// NormalizeDossier fills derived defaults without hiding invalid provider shape.
func NormalizeDossier(dossier Dossier) Dossier {
	for i := range dossier.Findings {
		if dossier.Findings[i].ID == "" {
			dossier.Findings[i].ID = fmt.Sprintf("finding-%d", i+1)
		}
		if dossier.Findings[i].Status == "" {
			dossier.Findings[i].Status = FindingOpen
		}
	}
	if dossier.Budget.ActualFindings == 0 && len(dossier.Findings) > 0 {
		dossier.Budget.ActualFindings = len(dossier.Findings)
	}
	if dossier.Budget.ActualAttackAngles == 0 && len(dossier.AttackLog) > 0 {
		dossier.Budget.ActualAttackAngles = len(dossier.AttackLog)
	}
	if dossier.Verdict == "" && len(dossier.Findings) > 0 {
		dossier.Verdict = VerdictFromFindings(dossier.Findings)
	}
	return dossier
}

// ValidateDossier verifies dossier shape and the completion gate contract.
func ValidateDossier(dossier Dossier) error {
	if !ValidVerdict(dossier.Verdict) {
		return fmt.Errorf("%w: invalid verdict %q", ErrInvalidDossier, dossier.Verdict)
	}
	if !ValidMode(dossier.Mode) {
		return fmt.Errorf("%w: invalid mode %q", ErrInvalidDossier, dossier.Mode)
	}
	if strings.TrimSpace(dossier.Summary) == "" {
		return fmt.Errorf("%w: summary is required", ErrInvalidDossier)
	}
	if len(dossier.AttackLog) == 0 {
		return fmt.Errorf("%w: attack_log must record at least one review attack", ErrInvalidDossier)
	}
	for _, attack := range dossier.AttackLog {
		if strings.TrimSpace(attack.Target) == "" || strings.TrimSpace(attack.Attack) == "" || strings.TrimSpace(string(attack.Result)) == "" {
			return fmt.Errorf("%w: attack_log entries require target, attack, and result", ErrInvalidDossier)
		}
		if !ValidAttackResult(attack.Result) {
			return fmt.Errorf("%w: invalid attack_log result %q", ErrInvalidDossier, attack.Result)
		}
	}
	for _, finding := range dossier.Findings {
		if err := ValidateFinding(finding); err != nil {
			return err
		}
	}
	derived := VerdictFromFindings(dossier.Findings)
	if dossier.Verdict != derived {
		return fmt.Errorf("%w: verdict %q contradicts findings verdict %q", ErrInvalidDossier, dossier.Verdict, derived)
	}
	return nil
}

// ValidateFinding checks one finding's typed fields and repair context.
func ValidateFinding(finding Finding) error {
	if strings.TrimSpace(finding.ID) == "" {
		return fmt.Errorf("%w: finding id is required", ErrInvalidDossier)
	}
	if !ValidSeverity(finding.Severity) {
		return fmt.Errorf("%w: invalid severity %q", ErrInvalidDossier, finding.Severity)
	}
	if finding.Confidence != "" && !ValidConfidence(finding.Confidence) {
		return fmt.Errorf("%w: invalid confidence %q", ErrInvalidDossier, finding.Confidence)
	}
	if finding.Status != "" && !ValidFindingStatus(finding.Status) {
		return fmt.Errorf("%w: invalid finding status %q", ErrInvalidDossier, finding.Status)
	}
	if BlocksCompletion(finding) {
		if finding.Location == nil || strings.TrimSpace(finding.Location.Path) == "" {
			return fmt.Errorf("%w: completion-blocking finding %q requires location", ErrInvalidDossier, finding.ID)
		}
		if strings.TrimSpace(finding.Evidence) == "" {
			return fmt.Errorf("%w: completion-blocking finding %q requires evidence", ErrInvalidDossier, finding.ID)
		}
		if strings.TrimSpace(finding.Impact) == "" {
			return fmt.Errorf("%w: completion-blocking finding %q requires impact", ErrInvalidDossier, finding.ID)
		}
		if strings.TrimSpace(finding.Validation) == "" {
			return fmt.Errorf("%w: completion-blocking finding %q requires validation", ErrInvalidDossier, finding.ID)
		}
	}
	return nil
}

// VerdictFromFindings derives the review verdict from open completion blockers.
func VerdictFromFindings(findings []Finding) string {
	for _, finding := range findings {
		if BlocksCompletion(finding) {
			return VerdictFail
		}
	}
	return VerdictPass
}

// ValidVerdict reports whether verdict is supported by the review gate.
func ValidVerdict(verdict string) bool {
	switch verdict {
	case VerdictPass, VerdictFail:
		return true
	default:
		return false
	}
}

// ValidMode reports whether mode is supported by the review gate.
func ValidMode(mode Mode) bool {
	switch mode {
	case ModeDiscover, ModeVerify:
		return true
	default:
		return false
	}
}

// ValidAttackResult reports whether an attack-log result is supported.
func ValidAttackResult(result AttackResult) bool {
	switch result {
	case AttackResultFinding, AttackResultClean, AttackResultSkipped:
		return true
	default:
		return false
	}
}

// ValidSeverity reports whether severity is supported by review dossiers.
func ValidSeverity(severity Severity) bool {
	switch severity {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow:
		return true
	default:
		return false
	}
}

// ValidConfidence reports whether confidence is supported by review dossiers.
func ValidConfidence(confidence Confidence) bool {
	switch confidence {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow:
		return true
	default:
		return false
	}
}

// ValidFindingStatus reports whether finding status is supported.
func ValidFindingStatus(status FindingStatus) bool {
	switch status {
	case FindingOpen, FindingFixed, FindingAcceptedRisk, FindingSuperseded:
		return true
	default:
		return false
	}
}
