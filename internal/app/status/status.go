package status

import (
	"context"
	"fmt"

	corecompletion "github.com/nilstate/scafld/v2/internal/core/completion"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec loading port used by status.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
}

// SessionStore is the session loading port used by status.
type SessionStore interface {
	Load(context.Context, string) (session.Session, error)
}

// Output describes the task status projection.
type Output struct {
	TaskID          string          `json:"task_id"`
	Status          spec.Status     `json:"status"`
	Title           string          `json:"title"`
	Next            string          `json:"next"`
	NextAction      NextAction      `json:"next_action,omitempty"`
	Gate            string          `json:"gate,omitempty"`
	TrustedState    string          `json:"trusted_state,omitempty"`
	AllowedFollowUp string          `json:"allowed_follow_up,omitempty"`
	SessionOK       bool            `json:"session_ok"`
	Repair          *gate.Failure   `json:"repair,omitempty"`
	Review          ReviewInfo      `json:"review,omitempty"`
	Completion      *CompletionInfo `json:"completion_authority,omitempty"`
}

// NextAction gives wrappers and trigger agents the next deterministic role and
// command without requiring them to interpret prose handoffs.
type NextAction struct {
	Role         string `json:"role,omitempty"`
	Action       string `json:"action,omitempty"`
	Command      string `json:"command,omitempty"`
	Reason       string `json:"reason,omitempty"`
	AfterCommand string `json:"after_command,omitempty"`
	ThenCommand  string `json:"then_command,omitempty"`
}

// ReviewInfo is the latest review evidence visible from status.
type ReviewInfo struct {
	Verdict        string                      `json:"verdict,omitempty"`
	Mode           corereview.Mode             `json:"mode,omitempty"`
	Summary        string                      `json:"summary,omitempty"`
	Findings       []corereview.Finding        `json:"findings,omitempty"`
	OpenBlockers   int                         `json:"open_blockers,omitempty"`
	AttackLog      []corereview.AttackLogEntry `json:"attack_log,omitempty"`
	Budget         corereview.Budget           `json:"budget,omitempty"`
	Provider       string                      `json:"provider,omitempty"`
	Model          string                      `json:"model,omitempty"`
	OutputFormat   string                      `json:"output_format,omitempty"`
	Normalizations []string                    `json:"normalizations,omitempty"`
	Attempt        *ReviewAttemptInfo          `json:"attempt,omitempty"`
	Running        bool                        `json:"running,omitempty"`
	AttemptStatus  string                      `json:"attempt_status,omitempty"`
	Reason         string                      `json:"reason,omitempty"`
}

// ReviewAttemptInfo describes the latest provider attempt separately from the
// latest accepted review verdict.
type ReviewAttemptInfo struct {
	Running bool   `json:"running"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
}

// CompletionInfo describes the terminal review authority for completed tasks.
type CompletionInfo struct {
	Status        string   `json:"status"`
	Kind          string   `json:"kind"`
	Provider      string   `json:"provider,omitempty"`
	Verdict       string   `json:"verdict,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	Actual        string   `json:"actual,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	ReviewEvent   string   `json:"review_event,omitempty"`
	ReceiptEvent  string   `json:"receipt_event,omitempty"`
	CompleteEvent string   `json:"complete_event,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
}

// Run reads status for taskID.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, taskID string) (Output, error) {
	model, _, err := specs.Load(ctx, taskID)
	if err != nil {
		return Output{}, err
	}
	out := Output{
		TaskID:          model.TaskID,
		Status:          model.Status,
		Title:           model.Title,
		Next:            model.CurrentState.AllowedFollowUp,
		Gate:            currentGate(model),
		TrustedState:    "session ledger replay projected into the Markdown spec",
		AllowedFollowUp: model.CurrentState.AllowedFollowUp,
	}
	if sessions != nil {
		if ledger, err := sessions.Load(ctx, model.TaskID); err == nil {
			out.SessionOK = true
			out.Review = latestReviewInfo(ledger)
			if model.Status == spec.StatusCompleted {
				out.Completion = completionInfo(corecompletion.TerminalAuthority(ledger))
			}
			out.Repair = repairContract(model, ledger)
			if out.Repair != nil && out.Repair.Next != "" {
				out.Next = out.Repair.Next
				out.AllowedFollowUp = out.Repair.Next
			}
		}
	}
	out.NextAction = nextAction(model, out.Repair, out.Review, out.Next)
	return out, nil
}

func nextAction(model spec.Model, repair *gate.Failure, review ReviewInfo, command string) NextAction {
	taskCommand := func(name string) string {
		if model.TaskID == "" {
			return "scafld " + name
		}
		return "scafld " + name + " " + model.TaskID
	}
	if command == "" {
		command = model.CurrentState.AllowedFollowUp
	}
	switch model.Status {
	case spec.StatusDraft:
		if model.HardenStatus == spec.HardenInProgress {
			return NextAction{Role: "planner", Action: "finish_hardening", Command: command, Reason: fallback(model.CurrentState.Reason, "draft hardening is in progress")}
		}
		return NextAction{Role: "operator", Action: "approve_contract", Command: command, Reason: "draft needs explicit approval before build"}
	case spec.StatusApproved:
		return NextAction{Role: "executor", Action: "open_build", Command: command, Reason: "approved contract is ready for phase execution"}
	case spec.StatusActive:
		return NextAction{Role: "executor", Action: "read_handoff", Command: command, Reason: fallback(model.CurrentState.Reason, "phase is open"), AfterCommand: taskCommand("build")}
	case spec.StatusBlocked:
		return NextAction{Role: "executor", Action: "repair_acceptance", Command: command, Reason: fallback(model.CurrentState.Reason, "acceptance is blocked"), AfterCommand: taskCommand("build")}
	case spec.StatusReview:
		if repair != nil && repair.Gate == "review" {
			if review.Verdict == corereview.VerdictFail {
				return NextAction{Role: "executor", Action: "repair_review_findings", Command: command, Reason: "review found completion blockers", AfterCommand: taskCommand("build"), ThenCommand: taskCommand("review")}
			}
			return NextAction{Role: "operator", Action: "repair_review_provider", Command: command, Reason: fallback(repair.Reason, "latest review attempt failed"), ThenCommand: taskCommand("review")}
		}
		if review.Verdict == corereview.VerdictPass {
			return NextAction{Role: "operator", Action: "complete", Command: command, Reason: "latest review gate passed"}
		}
		return NextAction{Role: "reviewer", Action: "run_review", Command: command, Reason: fallback(model.CurrentState.Reason, "build completed; ready for review")}
	case spec.StatusCompleted:
		return NextAction{Role: "none", Action: "done", Command: "none", Reason: "task is completed"}
	case spec.StatusFailed, spec.StatusCancelled:
		return NextAction{Role: "none", Action: "terminal", Command: "none", Reason: "task is terminal"}
	default:
		return NextAction{Role: "operator", Action: string(model.Status), Command: command, Reason: fallback(model.CurrentState.Reason, "inspect status")}
	}
}

func completionInfo(authority corecompletion.Authority) *CompletionInfo {
	if !authority.Completed {
		return nil
	}
	return &CompletionInfo{
		Status:        authority.Status(),
		Kind:          authority.Kind(),
		Provider:      authority.Provider(),
		Verdict:       authority.Verdict(),
		Reason:        authority.Reason,
		Actual:        authority.Actual,
		Summary:       authority.Summary(),
		ReviewEvent:   eventReference(authority.ReviewEntry),
		ReceiptEvent:  eventReference(authority.ReceiptEntry),
		CompleteEvent: eventReference(authority.CompleteEntry),
		Evidence:      authority.Evidence,
	}
}

func latestReviewInfo(ledger session.Session) ReviewInfo {
	var info ReviewInfo
	haveAttempt := false
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "review":
			info.Verdict = entry.Status
			info.Provider = entry.Provider
			info.Summary = entry.Reason
			if dossier, ok := corereview.DecodeDossier(entry.Output); ok {
				info.Mode = dossier.Mode
				info.Summary = dossier.Summary
				info.Findings = dossier.Findings
				info.OpenBlockers = corereview.OpenBlockerCount(dossier.Findings)
				info.AttackLog = dossier.AttackLog
				info.Budget = dossier.Budget
				info.Provider = dossier.Provider
				info.Model = dossier.Model
				info.OutputFormat = dossier.OutputFormat
				info.Normalizations = dossier.Normalizations
			}
			return info
		case "review_attempt":
			if haveAttempt {
				continue
			}
			info.Running = entry.Status == "running"
			info.AttemptStatus = entry.Status
			info.Reason = entry.Reason
			info.Attempt = &ReviewAttemptInfo{Running: info.Running, Status: entry.Status, Reason: entry.Reason}
			haveAttempt = true
		case "build", "criterion", "phase", "approval", session.EntryWorkspaceBaseline, "fail", "cancel":
			return info
		}
	}
	return info
}

func currentGate(model spec.Model) string {
	switch model.Status {
	case spec.StatusDraft:
		if model.HardenStatus == spec.HardenInProgress {
			return "harden"
		}
		return "approve"
	case spec.StatusApproved, spec.StatusActive, spec.StatusBlocked:
		return "build"
	case spec.StatusReview:
		return "review"
	case spec.StatusCompleted:
		return "complete"
	default:
		return string(model.Status)
	}
}

func repairContract(model spec.Model, ledger session.Session) *gate.Failure {
	switch model.Status {
	case spec.StatusBlocked:
		blockers := criterionBlockers(model)
		return &gate.Failure{
			Gate:     "build",
			Status:   string(model.Status),
			Reason:   model.CurrentState.Reason,
			Evidence: blockerEvidence(model, ledger),
			Expected: "all acceptance criteria pass",
			Actual:   fmt.Sprintf("%d blocker(s)", len(blockers)),
			Blockers: blockers,
			Next:     model.CurrentState.AllowedFollowUp,
		}
	case spec.StatusReview:
		review := latestReviewInfo(ledger)
		if review.AttemptStatus == "failed" {
			evidence := []string{"latest review_attempt session entry"}
			if attempt, ok := latestFailedReviewAttempt(ledger); ok && attempt.Path != "" {
				evidence = []string{attempt.Path}
			}
			return &gate.Failure{
				Gate:     "review",
				Status:   string(model.Status),
				Reason:   fallback(review.Reason, "latest review attempt failed"),
				Evidence: evidence,
				Expected: "valid ReviewDossier submitted by an external reviewer",
				Actual:   fallback(review.Reason, "review attempt failed"),
				Blockers: []string{fallback(review.Reason, "latest review attempt failed")},
				Next:     "scafld handoff " + model.TaskID,
			}
		}
		if review.Verdict == corereview.VerdictFail {
			return &gate.Failure{
				Gate:     "review",
				Status:   string(model.Status),
				Reason:   "review verdict fail",
				Evidence: []string{latestReviewEvidence(ledger)},
				Expected: "review verdict pass",
				Actual:   "review verdict fail",
				Blockers: reviewFindingSummaries(review.Findings),
				Next:     model.CurrentState.AllowedFollowUp,
			}
		}
	}
	return nil
}

func latestFailedReviewAttempt(ledger session.Session) (session.Entry, bool) {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "review_attempt":
			return entry, entry.Status == "failed"
		case "review", "build", "criterion", "phase", "approval", session.EntryWorkspaceBaseline, "fail", "cancel":
			return session.Entry{}, false
		}
	}
	return session.Entry{}, false
}

func fallback(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func latestReviewEvidence(ledger session.Session) string {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if entry.Type != "review" {
			continue
		}
		if entry.ID != "" {
			return entry.ID
		}
		return "review event"
	}
	return "review event"
}

func eventReference(entry session.Entry) string {
	if entry.ID != "" {
		return entry.ID
	}
	if entry.Type != "" {
		return entry.Type + " session entry"
	}
	return ""
}

func criterionBlockers(model spec.Model) []string {
	var blockers []string
	for _, criterion := range blockingCriteria(model) {
		if criterion.Status == "pass" {
			continue
		}
		label := criterion.ID
		if criterion.Title != "" {
			label += ": " + criterion.Title
		}
		if criterion.Evidence != "" {
			label += " (" + criterion.Evidence + ")"
		}
		blockers = append(blockers, label)
	}
	return blockers
}

func blockerEvidence(model spec.Model, ledger session.Session) []string {
	var evidence []string
	for _, criterion := range blockingCriteria(model) {
		if criterion.Status == "pass" || criterion.SourceEvent == "" {
			continue
		}
		evidence = append(evidence, evidenceReference(ledger, criterion.SourceEvent))
	}
	return evidence
}

func evidenceReference(ledger session.Session, sourceID string) string {
	for _, entry := range ledger.Entries {
		if entry.ID != sourceID {
			continue
		}
		if entry.Path != "" {
			return entry.Path
		}
		return sourceID
	}
	return sourceID
}

func blockingCriteria(model spec.Model) []spec.Criterion {
	switch model.CurrentState.CurrentPhase {
	case "", "none":
		return model.AllCriteria()
	case "final":
		return append([]spec.Criterion(nil), model.Acceptance.Criteria...)
	default:
		for _, phase := range model.Phases {
			if phase.ID == model.CurrentState.CurrentPhase {
				criteria := append([]spec.Criterion(nil), phase.Acceptance...)
				for i := range criteria {
					if criteria[i].PhaseID == "" {
						criteria[i].PhaseID = phase.ID
					}
				}
				return criteria
			}
		}
		return model.AllCriteria()
	}
}

func reviewFindingSummaries(findings []corereview.Finding) []string {
	blockers := make([]string, 0, len(findings))
	for _, finding := range findings {
		if !corereview.BlocksCompletion(finding) {
			continue
		}
		label := finding.ID
		if finding.Summary != "" {
			label += ": " + finding.Summary
		}
		if finding.Validation != "" {
			label += " | validate: " + finding.Validation
		}
		blockers = append(blockers, label)
	}
	return blockers
}
