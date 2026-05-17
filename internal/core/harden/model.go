// Package harden models provider-backed pre-approval hardening.
package harden

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
)

const (
	// VerdictPass means the draft contract is ready for approval.
	VerdictPass = "pass"
	// VerdictNeedsRevision means the draft needs contract edits before approval.
	VerdictNeedsRevision = "needs_revision"
)

// ErrInvalidDossier wraps malformed or semantically invalid harden output.
var ErrInvalidDossier = errors.New("invalid harden dossier")

// RequiredCheckNames are the evidence-backed hardening checks every round must record.
var RequiredCheckNames = []string{
	"path audit",
	"command audit",
	"scope/migration audit",
	"acceptance timing audit",
	"rollback/repair audit",
	"design challenge",
}

// Check records one required hardening check.
type Check struct {
	Name       string `json:"name"`
	GroundedIn string `json:"grounded_in"`
	Result     string `json:"result"`
	Evidence   string `json:"evidence"`
}

// Question records one grounded question the implementation agent must resolve.
type Question struct {
	Question          string `json:"question"`
	GroundedIn        string `json:"grounded_in"`
	RecommendedAnswer string `json:"recommended_answer"`
	IfUnanswered      string `json:"if_unanswered,omitempty"`
}

// DesignObjection challenges whether the draft is the right work at all.
type DesignObjection struct {
	ID             string `json:"id"`
	Severity       string `json:"severity"`
	GroundedIn     string `json:"grounded_in"`
	Summary        string `json:"summary"`
	Evidence       string `json:"evidence"`
	Recommendation string `json:"recommendation"`
}

// RecommendedEdit names a concrete spec edit needed before approval.
type RecommendedEdit struct {
	Section        string `json:"section"`
	GroundedIn     string `json:"grounded_in"`
	Recommendation string `json:"recommendation"`
}

// AttackLogEntry records one bounded attack angle used during hardening.
type AttackLogEntry struct {
	Target string `json:"target"`
	Attack string `json:"attack"`
	Result string `json:"result"`
	Notes  string `json:"notes,omitempty"`
}

// Dossier is the normalized harden-provider payload consumed by scafld.
type Dossier struct {
	Verdict          string            `json:"verdict"`
	Summary          string            `json:"summary"`
	Checks           []Check           `json:"checks"`
	Questions        []Question        `json:"questions"`
	DesignObjections []DesignObjection `json:"design_objections"`
	RecommendedEdits []RecommendedEdit `json:"recommended_edits"`
	AttackLog        []AttackLogEntry  `json:"attack_log"`
	Provider         string            `json:"provider,omitempty"`
	Model            string            `json:"model,omitempty"`
	SessionID        string            `json:"session_id,omitempty"`
	OutputFormat     string            `json:"output_format,omitempty"`
	EventSummary     map[string]int    `json:"event_summary,omitempty"`
	Raw              string            `json:"-"`
}

// Request is the provider-facing hardening prompt request.
type Request struct {
	TaskID  string
	Prompt  string
	Context reviewcontext.Packet
}

// EncodeDossier serializes a dossier for transport and storage.
func EncodeDossier(dossier Dossier) string {
	if strings.TrimSpace(dossier.Verdict) == "" && strings.TrimSpace(dossier.Summary) == "" && len(dossier.Checks) == 0 {
		return ""
	}
	data, err := json.Marshal(dossier)
	if err != nil {
		return ""
	}
	return string(data)
}

// ParseText parses one strict JSON HardenDossier.
func ParseText(text string) (Dossier, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Dossier{}, fmt.Errorf("%w: empty provider output", ErrInvalidDossier)
	}
	dossier, err := decodeDossierStrict([]byte(trimmed), text)
	if err != nil {
		return Dossier{}, err
	}
	if err := ValidateDossier(dossier); err != nil {
		return Dossier{}, err
	}
	return dossier, nil
}

func decodeDossierStrict(data []byte, raw string) (Dossier, error) {
	var dossier Dossier
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&dossier); err != nil {
		return Dossier{}, fmt.Errorf("%w: %v", ErrInvalidDossier, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Dossier{}, fmt.Errorf("%w: multiple JSON values", ErrInvalidDossier)
		}
		return Dossier{}, fmt.Errorf("%w: %v", ErrInvalidDossier, err)
	}
	dossier.Raw = raw
	return NormalizeDossier(dossier), nil
}

// NormalizeDossier fills derived defaults without hiding invalid provider shape.
func NormalizeDossier(dossier Dossier) Dossier {
	for i := range dossier.DesignObjections {
		if strings.TrimSpace(dossier.DesignObjections[i].ID) == "" {
			dossier.DesignObjections[i].ID = fmt.Sprintf("objection-%d", i+1)
		}
	}
	if strings.TrimSpace(dossier.Verdict) == "" {
		dossier.Verdict = VerdictFromDossier(dossier)
	}
	return dossier
}

// ValidateDossier verifies provider output shape and the harden gate contract.
func ValidateDossier(dossier Dossier) error {
	if !ValidVerdict(dossier.Verdict) {
		return fmt.Errorf("%w: invalid verdict %q", ErrInvalidDossier, dossier.Verdict)
	}
	if strings.TrimSpace(dossier.Summary) == "" {
		return fmt.Errorf("%w: summary is required", ErrInvalidDossier)
	}
	if len(dossier.AttackLog) == 0 {
		return fmt.Errorf("%w: attack_log must record at least one hardening attack", ErrInvalidDossier)
	}
	for _, attack := range dossier.AttackLog {
		if strings.TrimSpace(attack.Target) == "" || strings.TrimSpace(attack.Attack) == "" || strings.TrimSpace(attack.Result) == "" {
			return fmt.Errorf("%w: attack_log entries require target, attack, and result", ErrInvalidDossier)
		}
		if !validAttackResult(attack.Result) {
			return fmt.Errorf("%w: invalid attack_log result %q", ErrInvalidDossier, attack.Result)
		}
	}
	if err := validateChecks(dossier.Checks); err != nil {
		return err
	}
	for _, question := range dossier.Questions {
		if strings.TrimSpace(question.Question) == "" || strings.TrimSpace(question.GroundedIn) == "" || strings.TrimSpace(question.RecommendedAnswer) == "" {
			return fmt.Errorf("%w: questions require question, grounded_in, and recommended_answer", ErrInvalidDossier)
		}
	}
	for _, objection := range dossier.DesignObjections {
		if strings.TrimSpace(objection.ID) == "" || strings.TrimSpace(objection.Summary) == "" || strings.TrimSpace(objection.GroundedIn) == "" || strings.TrimSpace(objection.Evidence) == "" || strings.TrimSpace(objection.Recommendation) == "" {
			return fmt.Errorf("%w: design objections require id, summary, grounded_in, evidence, and recommendation", ErrInvalidDossier)
		}
		if !validSeverity(objection.Severity) {
			return fmt.Errorf("%w: invalid design objection severity %q", ErrInvalidDossier, objection.Severity)
		}
	}
	for _, edit := range dossier.RecommendedEdits {
		if strings.TrimSpace(edit.Section) == "" || strings.TrimSpace(edit.GroundedIn) == "" || strings.TrimSpace(edit.Recommendation) == "" {
			return fmt.Errorf("%w: recommended edits require section, grounded_in, and recommendation", ErrInvalidDossier)
		}
	}
	if dossier.Verdict != VerdictFromDossier(dossier) {
		return fmt.Errorf("%w: verdict %q contradicts harden findings verdict %q", ErrInvalidDossier, dossier.Verdict, VerdictFromDossier(dossier))
	}
	return nil
}

func validateChecks(checks []Check) error {
	if len(checks) == 0 {
		return fmt.Errorf("%w: checks must include required harden checks", ErrInvalidDossier)
	}
	seen := map[string]bool{}
	for _, check := range checks {
		name := normalize(check.Name)
		if name == "" || strings.TrimSpace(check.GroundedIn) == "" || strings.TrimSpace(check.Result) == "" || strings.TrimSpace(check.Evidence) == "" {
			return fmt.Errorf("%w: checks require name, grounded_in, result, and evidence", ErrInvalidDossier)
		}
		if !validCheckResult(check.Result) {
			return fmt.Errorf("%w: invalid check result %q", ErrInvalidDossier, check.Result)
		}
		seen[name] = true
	}
	var missing []string
	for _, required := range RequiredCheckNames {
		if !seen[required] {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing required harden checks: %s", ErrInvalidDossier, strings.Join(missing, ", "))
	}
	return nil
}

// VerdictFromDossier derives harden verdict from unresolved questions and failed checks.
func VerdictFromDossier(dossier Dossier) string {
	for _, check := range dossier.Checks {
		if strings.TrimSpace(strings.ToLower(check.Result)) == "failed" {
			return VerdictNeedsRevision
		}
	}
	if len(dossier.Questions) > 0 || len(dossier.DesignObjections) > 0 || len(dossier.RecommendedEdits) > 0 {
		return VerdictNeedsRevision
	}
	return VerdictPass
}

// ValidVerdict reports whether the harden verdict is supported.
func ValidVerdict(verdict string) bool {
	switch strings.TrimSpace(verdict) {
	case VerdictPass, VerdictNeedsRevision:
		return true
	default:
		return false
	}
}

func validCheckResult(result string) bool {
	switch strings.TrimSpace(strings.ToLower(result)) {
	case "passed", "failed", "not_applicable":
		return true
	default:
		return false
	}
}

func validAttackResult(result string) bool {
	switch strings.TrimSpace(strings.ToLower(result)) {
	case "finding", "clean", "skipped":
		return true
	default:
		return false
	}
}

func validSeverity(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "critical", "high", "medium", "low":
		return true
	default:
		return false
	}
}

func normalize(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}
