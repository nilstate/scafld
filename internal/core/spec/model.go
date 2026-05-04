package spec

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
)

type Status string

const (
	StatusDraft     Status = "draft"
	StatusApproved  Status = "approved"
	StatusActive    Status = "active"
	StatusBlocked   Status = "blocked"
	StatusReview    Status = "review"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type HardenStatus string

const (
	HardenNotRun HardenStatus = "not_run"
	HardenPassed HardenStatus = "passed"
	HardenFailed HardenStatus = "failed"
)

type Size string

const (
	SizeSmall  Size = "small"
	SizeMedium Size = "medium"
	SizeLarge  Size = "large"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

var (
	ErrInvalidSpec = errors.New("invalid spec")
	taskIDPattern  = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

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

type Record struct {
	TaskID string
	Status Status
	Path   string
	Title  string
}

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

type Acceptance struct {
	ValidationProfile string
	DefinitionDone    []ChecklistItem
	Criteria          []Criterion
}

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

type ChecklistItem struct {
	ID      string
	Text    string
	Checked bool
}

type CurrentState struct {
	CurrentPhase       string
	Next               string
	Reason             string
	Blockers           string
	AllowedFollowUp    string
	LatestRunnerUpdate string
	ReviewGate         string
}

type Context struct {
	CWD           string
	Packages      []string
	FilesImpacted []string
	Invariants    []string
	RelatedDocs   []string
}

type Risk struct {
	Description string
	Mitigation  string
}

type ReviewState struct {
	Status  string
	Verdict string
}

type Origin struct {
	CreatedBy string
	Source    string
}

type HardenRound struct {
	ID     string
	Status string
}

type PlanningEvent struct {
	Time string
	Text string
}

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

func Validate(model Model) Validation {
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
	seenPhase := map[string]bool{}
	for _, phase := range model.Phases {
		if phase.ID == "" {
			errs = append(errs, "phase id is required")
			continue
		}
		if seenPhase[phase.ID] {
			errs = append(errs, "duplicate phase id "+phase.ID)
		}
		seenPhase[phase.ID] = true
		if strings.TrimSpace(phase.Name) == "" {
			errs = append(errs, "phase "+phase.ID+" name is required")
		}
	}
	seenCriterion := map[string]bool{}
	checkCriterion := func(c Criterion) {
		if c.ID == "" {
			errs = append(errs, "criterion id is required")
			return
		}
		if seenCriterion[c.ID] {
			errs = append(errs, "duplicate criterion id "+c.ID)
		}
		seenCriterion[c.ID] = true
		if c.Command == "" && c.Type != "manual" {
			errs = append(errs, "criterion "+c.ID+" command is required")
		}
		if !acceptance.ValidExpectedKind(c.ExpectedKind) {
			errs = append(errs, "criterion "+c.ID+" expected_kind is invalid")
		}
	}
	for _, c := range model.Acceptance.Criteria {
		checkCriterion(c)
	}
	for _, phase := range model.Phases {
		for _, c := range phase.Acceptance {
			if c.PhaseID == "" {
				c.PhaseID = phase.ID
			}
			checkCriterion(c)
		}
	}
	return Validation{Valid: len(errs) == 0, Errors: errs}
}

func ValidStatus(status Status) bool {
	switch status {
	case StatusDraft, StatusApproved, StatusActive, StatusBlocked, StatusReview, StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

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

func (m Model) WithStatus(status Status) Model {
	next := m
	next.Status = status
	next.CurrentState.Next = string(status)
	return next
}
