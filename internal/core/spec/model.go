package spec

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
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
	// HardenFailed means hardening identified unresolved blockers.
	HardenFailed HardenStatus = "failed"
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
	Version      string
	TaskID       string
	Created      string
	Updated      string
	Title        string
	Summary      string
	Status       Status
	HardenStatus HardenStatus
	Size         Size
	RiskLevel    RiskLevel
	CurrentState CurrentState
	Context      Context
	Objectives   []string
	Scope        []string
	Dependencies []string
	Assumptions  []string
	Touchpoints  []string
	Risks        []Risk
	Acceptance   Acceptance
	Phases       []Phase
	Rollback     []string
	Review       ReviewState
	SelfEval     []string
	Deviations   []string
	Metadata     map[string]string
	Origin       Origin
	HardenRounds []HardenRound
	PlanningLog  []PlanningEvent
}

// Record is a compact listing entry for a task spec.
type Record struct {
	TaskID string
	Status Status
	Path   string
	Title  string
}

// Phase describes a numbered execution phase and its evidence-derived state.
type Phase struct {
	ID             string
	Number         int
	Name           string
	Status         string
	Reason         string
	Dependencies   []string
	Objective      string
	Changes        []string
	Acceptance     []Criterion
	DefinitionDone []ChecklistItem
}

// Acceptance groups global definition-of-done and criterion checks.
type Acceptance struct {
	ValidationProfile string
	DefinitionDone    []ChecklistItem
	Criteria          []Criterion
}

// Criterion is a machine-checkable acceptance item.
type Criterion struct {
	ID           string
	Title        string
	Type         string
	PhaseID      string
	Command      string
	ExpectedKind acceptance.ExpectedKind
	Status       string
	Evidence     string
	SourceEvent  string
}

// ChecklistItem is one human-readable checklist row.
type ChecklistItem struct {
	ID      string
	Text    string
	Checked bool
}

// CurrentState is the readable projection of the next task action.
type CurrentState struct {
	CurrentPhase       string
	Next               string
	Reason             string
	Blockers           string
	AllowedFollowUp    string
	LatestRunnerUpdate string
	ReviewGate         string
}

// Context captures workspace and codebase surfaces relevant to the task.
type Context struct {
	CWD           string
	Packages      []string
	FilesImpacted []string
	Invariants    []string
	RelatedDocs   []string
}

// Risk captures one known risk and its mitigation.
type Risk struct {
	Description string
	Mitigation  string
}

// ReviewState stores the latest review gate projection.
type ReviewState struct {
	Status  string
	Verdict string
}

// Origin records where the spec came from.
type Origin struct {
	CreatedBy string
	Source    string
}

// HardenRound records one pre-approval hardening pass.
type HardenRound struct {
	ID        string
	Status    string
	StartedAt string
	EndedAt   string
	Questions []HardenQuestion
}

// HardenQuestion is one grounded question from a hardening round.
type HardenQuestion struct {
	Question          string
	GroundedIn        string
	RecommendedAnswer string
	IfUnanswered      string
	AnsweredWith      string
}

// PlanningEvent records a timestamped planning log entry.
type PlanningEvent struct {
	Time string
	Text string
}

// Validation reports whether a spec model satisfies runtime requirements.
type Validation struct {
	Valid  bool
	Errors []string
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
	case "", HardenNotRun, HardenInProgress, HardenPassed, HardenFailed:
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
