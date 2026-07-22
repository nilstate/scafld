package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
)

// Status names a task's lifecycle state in the normalized spec model.
type Status string

const (
	// StatusDraft is the editable pre-approval state.
	StatusDraft Status = "draft"
	// StatusApproved is ready for execution.
	StatusApproved Status = "approved"
	// StatusActive is actively executing acceptance criteria.
	StatusActive Status = "active"
	// StatusBlocked means execution or review found blocking work.
	StatusBlocked Status = "blocked"
	// StatusReview is waiting on or inside the review gate.
	StatusReview Status = "review"
	// StatusCompleted is the terminal successful state.
	StatusCompleted Status = "completed"
	// StatusFailed is the terminal unsuccessful state.
	StatusFailed Status = "failed"
	// StatusCancelled is the terminal abandoned state.
	StatusCancelled Status = "cancelled"
)

// HardenStatus names the state of the pre-approval hardening pass.
type HardenStatus string

const (
	// HardenNotRun means no hardening round has been opened.
	HardenNotRun HardenStatus = "not_run"
	// HardenInProgress means a hardening round is waiting for answers.
	HardenInProgress HardenStatus = "in_progress"
	// HardenPassed means hardening completed successfully.
	HardenPassed HardenStatus = "passed"
	// HardenNeedsRevision means hardening identified draft contract work before approval.
	HardenNeedsRevision HardenStatus = "needs_revision"
	// HardenOverridden means the operator approved the draft despite incomplete or rejected harden evidence.
	HardenOverridden HardenStatus = "overridden"
	// HardenError means the hardening provider or recorded harden evidence could not be accepted.
	HardenError HardenStatus = "error"
)

// Size estimates implementation effort.
type Size string

const (
	// SizeSmall describes a small task.
	SizeSmall Size = "small"
	// SizeMedium describes a medium task.
	SizeMedium Size = "medium"
	// SizeLarge describes a large task.
	SizeLarge Size = "large"
)

// RiskLevel estimates implementation risk.
type RiskLevel string

const (
	// RiskLow describes low implementation risk.
	RiskLow RiskLevel = "low"
	// RiskMedium describes medium implementation risk.
	RiskMedium RiskLevel = "medium"
	// RiskHigh describes high implementation risk.
	RiskHigh RiskLevel = "high"
)

var (
	// ErrInvalidSpec wraps normalized spec validation failures.
	ErrInvalidSpec = errors.New("invalid spec")
	taskIDPattern  = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

// Model is the normalized in-memory representation of a living Markdown spec.
type Model struct {
	Version      string            `json:"spec_version"`
	TaskID       string            `json:"task_id"`
	Created      string            `json:"created"`
	Updated      string            `json:"updated"`
	Title        string            `json:"title"`
	Summary      string            `json:"summary"`
	Status       Status            `json:"status"`
	HardenStatus HardenStatus      `json:"harden_status"`
	Size         Size              `json:"size"`
	RiskLevel    RiskLevel         `json:"risk_level"`
	CurrentState CurrentState      `json:"current_state"`
	Context      Context           `json:"context"`
	Objectives   []string          `json:"objectives"`
	Scope        []string          `json:"scope"`
	Dependencies []string          `json:"dependencies"`
	Assumptions  []string          `json:"assumptions"`
	Touchpoints  []string          `json:"touchpoints"`
	Risks        []Risk            `json:"risks"`
	Acceptance   Acceptance        `json:"acceptance"`
	Phases       []Phase           `json:"phases"`
	Rollback     []string          `json:"rollback"`
	Review       ReviewState       `json:"review"`
	SelfEval     []string          `json:"self_eval"`
	Deviations   []string          `json:"deviations"`
	Metadata     map[string]string `json:"metadata"`
	Origin       Origin            `json:"origin"`
	HardenRounds []HardenRound     `json:"harden_rounds"`
	PlanningLog  []PlanningEvent   `json:"planning_log"`
}

// Record is a compact listing entry for a task spec.
type Record struct {
	TaskID string `json:"task_id"`
	Status Status `json:"status"`
	Path   string `json:"path"`
	Title  string `json:"title"`
}

// Source carries the parsed model and exact Markdown bytes it came from.
type Source struct {
	Model    Model  `json:"model"`
	Path     string `json:"path"`
	Markdown []byte `json:"-"`
}

// Phase describes a numbered execution phase and its evidence-derived state.
type Phase struct {
	ID             string          `json:"id"`
	Number         int             `json:"number"`
	Name           string          `json:"name"`
	Status         string          `json:"status"`
	Reason         string          `json:"reason"`
	Dependencies   []string        `json:"dependencies"`
	Objective      string          `json:"objective"`
	Changes        []string        `json:"changes"`
	Acceptance     []Criterion     `json:"acceptance"`
	DefinitionDone []ChecklistItem `json:"definition_done"`
}

// Acceptance groups global definition-of-done and criterion checks.
type Acceptance struct {
	ValidationProfile string          `json:"validation_profile"`
	DefinitionDone    []ChecklistItem `json:"definition_done"`
	Criteria          []Criterion     `json:"criteria"`
}

// Criterion is a machine-checkable acceptance item.
type Criterion struct {
	ID           string                  `json:"id"`
	Title        string                  `json:"title"`
	Type         string                  `json:"type"`
	PhaseID      string                  `json:"phase_id"`
	Command      string                  `json:"command"`
	ExpectedKind acceptance.ExpectedKind `json:"expected_kind"`
	Status       string                  `json:"status"`
	Evidence     string                  `json:"evidence"`
	SourceEvent  string                  `json:"source_event"`
}

// ChecklistItem is one human-readable checklist row.
type ChecklistItem struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
}

// CurrentState is the readable projection of the next task action.
type CurrentState struct {
	CurrentPhase       string `json:"current_phase"`
	Next               string `json:"next"`
	Reason             string `json:"reason"`
	Blockers           string `json:"blockers"`
	AllowedFollowUp    string `json:"allowed_follow_up"`
	LatestRunnerUpdate string `json:"latest_runner_update"`
	ReviewGate         string `json:"review_gate"`
}

// Context captures workspace and codebase surfaces relevant to the task.
type Context struct {
	CWD           string   `json:"cwd"`
	Packages      []string `json:"packages"`
	FilesImpacted []string `json:"files_impacted"`
	Invariants    []string `json:"invariants"`
	RelatedDocs   []string `json:"related_docs"`
}

// Risk captures one known risk and its mitigation.
type Risk struct {
	Description string `json:"description"`
	Mitigation  string `json:"mitigation"`
}

// ReviewState stores the latest review gate projection.
type ReviewState struct {
	Status         string                      `json:"status"`
	Verdict        string                      `json:"verdict"`
	Mode           corereview.Mode             `json:"mode,omitempty"`
	Summary        string                      `json:"summary,omitempty"`
	Findings       []corereview.Finding        `json:"findings,omitempty"`
	AttackLog      []corereview.AttackLogEntry `json:"attack_log,omitempty"`
	Budget         corereview.Budget           `json:"budget,omitempty"`
	Provider       string                      `json:"provider,omitempty"`
	Model          string                      `json:"model,omitempty"`
	OutputFormat   string                      `json:"output_format,omitempty"`
	Normalizations []string                    `json:"normalizations,omitempty"`
}

// Origin records where the spec came from.
type Origin struct {
	CreatedBy string `json:"created_by"`
	Source    string `json:"source"`
}

// HardenRound records one pre-approval hardening pass.
type HardenRound struct {
	ID             string              `json:"id"`
	Status         string              `json:"status"`
	StartedAt      string              `json:"started_at"`
	EndedAt        string              `json:"ended_at"`
	SpecDigest     string              `json:"spec_digest,omitempty"`
	Verdict        string              `json:"verdict,omitempty"`
	Summary        string              `json:"summary,omitempty"`
	DiagnosticPath string              `json:"diagnostic_path,omitempty"`
	Provider       string              `json:"provider,omitempty"`
	Model          string              `json:"model,omitempty"`
	OutputFormat   string              `json:"output_format,omitempty"`
	Shape          HardenShape         `json:"shape,omitempty"`
	Observations   []HardenObservation `json:"observations"`
}

// HardenShape records the harden gate's answer to whether the draft should
// exist in its current form.
type HardenShape struct {
	Decision          string   `json:"decision,omitempty"`
	TrueShape         string   `json:"true_shape,omitempty"`
	MinimalPlan       string   `json:"minimal_plan,omitempty"`
	SharedOwner       string   `json:"shared_owner,omitempty"`
	AdapterBoundaries []string `json:"adapter_boundaries,omitempty"`
	RequiredSpecEdits []string `json:"required_spec_edits,omitempty"`
}

// HardenObservation records one grounded hardening observation.
type HardenObservation struct {
	Dimension    string `json:"dimension"`
	Result       string `json:"result"`
	Anchor       string `json:"anchor"`
	Note         string `json:"note,omitempty"`
	Question     string `json:"question,omitempty"`
	Recommended  string `json:"recommended,omitempty"`
	IfUnanswered string `json:"if_unanswered,omitempty"`
	Default      string `json:"default,omitempty"`
	Status       string `json:"status,omitempty"`
}

// PlanningEvent records a timestamped planning log entry.
type PlanningEvent struct {
	Time string `json:"time"`
	Text string `json:"text"`
}

// Validation reports whether a spec model satisfies runtime requirements.
type Validation struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

func (v Validation) Error() string {
	if len(v.Errors) == 0 {
		return ErrInvalidSpec.Error()
	}
	return fmt.Sprintf("%s: %s", ErrInvalidSpec, strings.Join(v.Errors, "; "))
}

func (v Validation) Unwrap() error {
	return ErrInvalidSpec
}

// Validate checks model shape and executable acceptance semantics.
func Validate(model Model) Validation {
	var errs []string
	errs = append(errs, validateModelFields(model)...)
	errs = append(errs, validatePhases(model.Phases)...)
	seenCriterion := map[string]bool{}
	for _, c := range model.Acceptance.Criteria {
		errs = append(errs, validateCriterion(c, seenCriterion)...)
	}
	for _, phase := range model.Phases {
		for _, c := range phase.Acceptance {
			if c.PhaseID == "" {
				c.PhaseID = phase.ID
			}
			errs = append(errs, validateCriterion(c, seenCriterion)...)
		}
	}
	return Validation{Valid: len(errs) == 0, Errors: errs}
}

func validateModelFields(model Model) []string {
	var errs []string
	if model.Version != "2.0" {
		errs = append(errs, "spec_version must be 2.0")
	}
	if !taskIDPattern.MatchString(model.TaskID) {
		errs = append(errs, "task_id must match [a-z][a-z0-9_-]*")
	}
	if strings.TrimSpace(model.Title) == "" {
		errs = append(errs, "title is required")
	}
	if !ValidStatus(model.Status) {
		errs = append(errs, "status is invalid")
	}
	if !ValidHardenStatus(model.HardenStatus) {
		errs = append(errs, "harden_status is invalid")
	}
	return errs
}

func validatePhases(phases []Phase) []string {
	var errs []string
	seen := map[string]bool{}
	for _, phase := range phases {
		errs = append(errs, validatePhase(phase, seen)...)
	}
	return errs
}

func validatePhase(phase Phase, seen map[string]bool) []string {
	if phase.ID == "" {
		return []string{"phase id is required"}
	}
	var errs []string
	if seen[phase.ID] {
		errs = append(errs, "duplicate phase id "+phase.ID)
	}
	seen[phase.ID] = true
	if strings.TrimSpace(phase.Name) == "" {
		errs = append(errs, "phase "+phase.ID+" name is required")
	}
	return errs
}

func validateCriterion(c Criterion, seen map[string]bool) []string {
	if c.ID == "" {
		return []string{"criterion id is required"}
	}
	var errs []string
	if seen[c.ID] {
		errs = append(errs, "duplicate criterion id "+c.ID)
	}
	seen[c.ID] = true
	c.Type = canonicalCriterionType(c.Type)
	if c.Command == "" && c.Type != "manual" {
		errs = append(errs, "criterion "+c.ID+" command is required")
	}
	if !acceptance.ValidExpectedKind(c.ExpectedKind) {
		errs = append(errs, fmt.Sprintf("criterion %s expected_kind %q is invalid; supported values: %s%s", c.ID, c.ExpectedKind, expectedKindList(), expectedKindHint(c.ExpectedKind)))
	}
	if c.Type == "browser" && c.ExpectedKind != acceptance.ExpectedBrowserEvidence {
		errs = append(errs, "criterion "+c.ID+" browser type requires expected_kind browser_evidence")
	}
	if c.ExpectedKind == acceptance.ExpectedBrowserEvidence && c.Type != "browser" {
		errs = append(errs, "criterion "+c.ID+" browser_evidence expected_kind requires browser type")
	}
	if c.Type == "manual" && c.ExpectedKind != acceptance.ExpectedManual {
		errs = append(errs, "criterion "+c.ID+" manual type requires expected_kind manual")
	}
	if c.ExpectedKind == acceptance.ExpectedManual && c.Type != "manual" {
		errs = append(errs, "criterion "+c.ID+" manual expected_kind requires manual type")
	}
	if c.Type == "manual" && strings.TrimSpace(c.Command) != "" {
		errs = append(errs, "criterion "+c.ID+" manual type cannot have a command")
	}
	return errs
}

func expectedKindList() string {
	values := acceptance.ExpectedKindValues()
	names := make([]string, 0, len(values))
	for _, value := range values {
		names = append(names, string(value))
	}
	return strings.Join(names, ", ")
}

func expectedKindHint(kind acceptance.ExpectedKind) string {
	switch strings.TrimSpace(string(kind)) {
	case "stdout_empty", "empty_stdout":
		return "; use no_matches when stdout must be empty"
	default:
		return ""
	}
}

// ValidStatus reports whether status is a supported lifecycle status.
func ValidStatus(status Status) bool {
	switch status {
	case StatusDraft, StatusApproved, StatusActive, StatusBlocked, StatusReview, StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

// ValidHardenStatus reports whether status is a supported hardening status.
func ValidHardenStatus(status HardenStatus) bool {
	switch status {
	case "", HardenNotRun, HardenInProgress, HardenPassed, HardenNeedsRevision, HardenOverridden, HardenError:
		return true
	default:
		return false
	}
}

// AllCriteria returns global and phase acceptance criteria with phase IDs filled.
func (m Model) AllCriteria() []Criterion {
	criteria := append([]Criterion(nil), m.Acceptance.Criteria...)
	for _, phase := range m.Phases {
		for _, criterion := range phase.Acceptance {
			if criterion.PhaseID == "" {
				criterion.PhaseID = phase.ID
			}
			criteria = append(criteria, criterion)
		}
	}
	return criteria
}

type contractChecklistItem struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type contractCriterion struct {
	ID           string                  `json:"id"`
	Title        string                  `json:"title"`
	Type         string                  `json:"type"`
	PhaseID      string                  `json:"phase_id,omitempty"`
	Command      string                  `json:"command"`
	ExpectedKind acceptance.ExpectedKind `json:"expected_kind"`
}

type contractAcceptance struct {
	ValidationProfile string                  `json:"validation_profile"`
	DefinitionDone    []contractChecklistItem `json:"definition_done"`
	Criteria          []contractCriterion     `json:"criteria"`
}

type contractPhase struct {
	ID             string                  `json:"id"`
	Number         int                     `json:"number"`
	Name           string                  `json:"name"`
	Dependencies   []string                `json:"dependencies"`
	Objective      string                  `json:"objective"`
	Changes        []string                `json:"changes"`
	Acceptance     []contractCriterion     `json:"acceptance"`
	DefinitionDone []contractChecklistItem `json:"definition_done"`
}

func acceptanceContract(acceptance Acceptance) contractAcceptance {
	return contractAcceptance{
		ValidationProfile: acceptance.ValidationProfile,
		DefinitionDone:    checklistContract(acceptance.DefinitionDone),
		Criteria:          criteriaContract(acceptance.Criteria, ""),
	}
}

func phasesContract(phases []Phase) []contractPhase {
	out := make([]contractPhase, 0, len(phases))
	for _, phase := range phases {
		out = append(out, contractPhase{
			ID:             phase.ID,
			Number:         phase.Number,
			Name:           phase.Name,
			Dependencies:   phase.Dependencies,
			Objective:      phase.Objective,
			Changes:        phase.Changes,
			Acceptance:     criteriaContract(phase.Acceptance, phase.ID),
			DefinitionDone: checklistContract(phase.DefinitionDone),
		})
	}
	return out
}

func criteriaContract(criteria []Criterion, defaultPhaseID string) []contractCriterion {
	out := make([]contractCriterion, 0, len(criteria))
	for _, criterion := range criteria {
		phaseID := criterion.PhaseID
		if phaseID == "" {
			phaseID = defaultPhaseID
		}
		out = append(out, contractCriterion{
			ID:           criterion.ID,
			Title:        criterion.Title,
			Type:         canonicalCriterionType(criterion.Type),
			PhaseID:      phaseID,
			Command:      criterion.Command,
			ExpectedKind: canonicalExpectedKind(criterion),
		})
	}
	return out
}

func checklistContract(items []ChecklistItem) []contractChecklistItem {
	out := make([]contractChecklistItem, 0, len(items))
	for _, item := range items {
		out = append(out, contractChecklistItem{ID: item.ID, Text: item.Text})
	}
	return out
}

func canonicalCriterionType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "command"
	}
	return value
}

func canonicalExpectedKind(criterion Criterion) acceptance.ExpectedKind {
	if criterion.ExpectedKind != "" {
		return criterion.ExpectedKind
	}
	switch canonicalCriterionType(criterion.Type) {
	case "browser":
		return acceptance.ExpectedBrowserEvidence
	case "manual":
		return acceptance.ExpectedManual
	default:
		return acceptance.ExpectedExitCodeZero
	}
}

// ContractDigest returns a stable digest of the task contract under review.
// Volatile projection fields such as lifecycle status, current state, updated
// timestamps, harden/review output, and acceptance evidence are intentionally
// excluded.
func ContractDigest(model Model) string {
	data, err := json.Marshal(struct {
		Version      string             `json:"spec_version"`
		TaskID       string             `json:"task_id"`
		Title        string             `json:"title"`
		Summary      string             `json:"summary"`
		Size         Size               `json:"size"`
		RiskLevel    RiskLevel          `json:"risk_level"`
		Context      Context            `json:"context"`
		Objectives   []string           `json:"objectives"`
		Scope        []string           `json:"scope"`
		Dependencies []string           `json:"dependencies"`
		Assumptions  []string           `json:"assumptions"`
		Touchpoints  []string           `json:"touchpoints"`
		Risks        []Risk             `json:"risks"`
		Acceptance   contractAcceptance `json:"acceptance"`
		Phases       []contractPhase    `json:"phases"`
		Rollback     []string           `json:"rollback"`
		SelfEval     []string           `json:"self_eval"`
		Deviations   []string           `json:"deviations"`
		Metadata     map[string]string  `json:"metadata"`
	}{
		Version:      model.Version,
		TaskID:       model.TaskID,
		Title:        model.Title,
		Summary:      model.Summary,
		Size:         model.Size,
		RiskLevel:    model.RiskLevel,
		Context:      model.Context,
		Objectives:   model.Objectives,
		Scope:        model.Scope,
		Dependencies: model.Dependencies,
		Assumptions:  model.Assumptions,
		Touchpoints:  model.Touchpoints,
		Risks:        model.Risks,
		Acceptance:   acceptanceContract(model.Acceptance),
		Phases:       phasesContract(model.Phases),
		Rollback:     model.Rollback,
		SelfEval:     model.SelfEval,
		Deviations:   model.Deviations,
		Metadata:     model.Metadata,
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// HardenContractDigest returns a stable digest of the draft material under the
// pre-approval harden gate. It excludes lifecycle projections and harden evidence
// so rerunning harden without changing the draft contract is detectable.
func HardenContractDigest(model Model) string {
	data, err := json.Marshal(struct {
		Version      string             `json:"spec_version"`
		TaskID       string             `json:"task_id"`
		Title        string             `json:"title"`
		Summary      string             `json:"summary"`
		Size         Size               `json:"size"`
		RiskLevel    RiskLevel          `json:"risk_level"`
		Context      Context            `json:"context"`
		Objectives   []string           `json:"objectives"`
		Scope        []string           `json:"scope"`
		Dependencies []string           `json:"dependencies"`
		Assumptions  []string           `json:"assumptions"`
		Touchpoints  []string           `json:"touchpoints"`
		Risks        []Risk             `json:"risks"`
		Acceptance   contractAcceptance `json:"acceptance"`
		Phases       []contractPhase    `json:"phases"`
		Rollback     []string           `json:"rollback"`
		SelfEval     []string           `json:"self_eval"`
		Deviations   []string           `json:"deviations"`
		Metadata     map[string]string  `json:"metadata"`
	}{
		Version:      model.Version,
		TaskID:       model.TaskID,
		Title:        model.Title,
		Summary:      model.Summary,
		Size:         model.Size,
		RiskLevel:    model.RiskLevel,
		Context:      model.Context,
		Objectives:   model.Objectives,
		Scope:        model.Scope,
		Dependencies: model.Dependencies,
		Assumptions:  model.Assumptions,
		Touchpoints:  model.Touchpoints,
		Risks:        model.Risks,
		Acceptance:   acceptanceContract(model.Acceptance),
		Phases:       phasesContract(model.Phases),
		Rollback:     model.Rollback,
		SelfEval:     model.SelfEval,
		Deviations:   model.Deviations,
		Metadata:     model.Metadata,
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// WithStatus returns a copy of the model with status and next state updated.
func (m Model) WithStatus(status Status) Model {
	next := m
	next.Status = status
	next.CurrentState.Next = string(status)
	return next
}
