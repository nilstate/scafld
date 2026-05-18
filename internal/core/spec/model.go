package spec

import (
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
	ID           string        `json:"id"`
	Status       string        `json:"status"`
	StartedAt    string        `json:"started_at"`
	EndedAt      string        `json:"ended_at"`
	Verdict      string        `json:"verdict,omitempty"`
	Summary      string        `json:"summary,omitempty"`
	Provider     string        `json:"provider,omitempty"`
	Model        string        `json:"model,omitempty"`
	OutputFormat string        `json:"output_format,omitempty"`
	Checks       []HardenCheck `json:"checks"`
	Issues       []HardenIssue `json:"issues,omitempty"`
}

// HardenCheck records one evidence-backed audit pass from a hardening round.
type HardenCheck struct {
	Name       string `json:"name"`
	GroundedIn string `json:"grounded_in"`
	Result     string `json:"result"`
	Evidence   string `json:"evidence"`
}

// HardenIssue records one approval blocker or non-blocking harden advisory.
type HardenIssue struct {
	ID                string `json:"id"`
	Kind              string `json:"kind"`
	Severity          string `json:"severity"`
	BlocksApproval    bool   `json:"blocks_approval"`
	Status            string `json:"status"`
	GroundedIn        string `json:"grounded_in"`
	Summary           string `json:"summary"`
	Evidence          string `json:"evidence"`
	Recommendation    string `json:"recommendation"`
	Question          string `json:"question,omitempty"`
	RecommendedAnswer string `json:"recommended_answer,omitempty"`
	IfUnanswered      string `json:"if_unanswered,omitempty"`
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
	if c.Command == "" && c.Type != "manual" {
		errs = append(errs, "criterion "+c.ID+" command is required")
	}
	if !acceptance.ValidExpectedKind(c.ExpectedKind) {
		errs = append(errs, "criterion "+c.ID+" expected_kind is invalid")
	}
	if c.Type == "browser" && c.ExpectedKind != acceptance.ExpectedBrowserEvidence {
		errs = append(errs, "criterion "+c.ID+" browser type requires expected_kind browser_evidence")
	}
	if c.ExpectedKind == acceptance.ExpectedBrowserEvidence && c.Type != "browser" {
		errs = append(errs, "criterion "+c.ID+" browser_evidence expected_kind requires browser type")
	}
	return errs
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
	case "", HardenNotRun, HardenInProgress, HardenPassed, HardenNeedsRevision, HardenError:
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

// WithStatus returns a copy of the model with status and next state updated.
func (m Model) WithStatus(status Status) Model {
	next := m
	next.Status = status
	next.CurrentState.Next = string(status)
	return next
}
