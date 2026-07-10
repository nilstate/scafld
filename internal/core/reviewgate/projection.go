package reviewgate

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

const (
	// EntryReviewAttempt records a leased provider review lifecycle attempt.
	EntryReviewAttempt = "review_attempt"

	AttemptStatusRunning   = "running"
	AttemptStatusAccepted  = "accepted"
	AttemptStatusFailed    = "failed"
	AttemptStatusAbandoned = "abandoned"

	DefaultAttemptLease = 2 * time.Hour
)

// Kind is the stable review gate projection state.
type Kind string

const (
	KindReadyForReview            Kind = "ready_for_review"
	KindAttemptRunning            Kind = "attempt_running"
	KindAttemptStale              Kind = "attempt_stale"
	KindAttemptFailed             Kind = "attempt_failed"
	KindReviewFailed              Kind = "review_failed"
	KindReviewPassed              Kind = "review_passed"
	KindReviewStaleAfterBuild          = "review_stale_after_build"
	KindReviewStaleAfterSpec           = "review_stale_after_spec"
	KindReviewStaleAfterWorkspace      = "review_stale_after_workspace"
	KindCompletedAuthorized            = "completed_authorized"
	KindInvalid                        = "invalid"
)

// WorkspaceSeal is the current workspace seal used to decide whether a passing
// review still covers the files being completed.
type WorkspaceSeal struct {
	Head  string
	Dirty string
	Diff  string
}

// Options configures review gate projection.
type Options struct {
	Now              time.Time
	AttemptLease     time.Duration
	WorkspaceSeal    WorkspaceSeal
	HasWorkspaceSeal bool
	MaterialSeal     reviewevidence.MaterialSeal
	HasMaterialSeal  bool
}

// State is the single read model for review gate lifecycle decisions.
type State struct {
	Kind                    Kind
	TaskID                  string
	Reason                  string
	Actual                  string
	Next                    string
	Evidence                []string
	Blockers                []string
	CanStartReview          bool
	ShouldAbandonAttempt    bool
	ReviewBlockedUntilBuild bool
	Authority               Authority
	LatestAttempt           Attempt
	HasAttempt              bool
	LatestReview            session.Entry
	HasReview               bool
	Dossier                 corereview.Dossier
	HasDossier              bool
}

// Attempt is normalized metadata for the latest current review attempt.
type Attempt struct {
	Entry          session.Entry
	AttemptID      string
	Status         string
	Running        bool
	Stale          bool
	LeaseExpiresAt time.Time
	HasLease       bool
	Provider       string
	Model          string
	Mode           corereview.Mode
	PassCount      int
	Reason         string
	DiagnosticPath string
}

// AttemptEntryInput contains the typed metadata stamped onto review_attempt entries.
type AttemptEntryInput struct {
	AttemptID      string
	LeaseExpiresAt time.Time
	Mode           corereview.Mode
	Provider       string
	Model          string
	PassCount      int
	Reason         string
	Output         string
}

// Project reduces a session ledger plus current task state into the review
// gate state. Readers should treat this as read-only; mutating commands append
// recovery entries based on the projected state.
func Project(ledger session.Session, model spec.Model, opts Options) State {
	opts = normalizeOptions(opts)
	taskID := firstNonBlank(model.TaskID, ledger.TaskID)
	state := State{
		Kind:           KindReadyForReview,
		TaskID:         taskID,
		Next:           reviewCommand(taskID),
		CanStartReview: model.Status == spec.StatusReview,
		Authority:      CurrentReviewGate(ledger),
	}
	if entry, dossier, hasReview, hasDossier := latestVisibleReview(ledger); hasReview {
		state.LatestReview = entry
		state.HasReview = true
		state.Dossier = dossier
		state.HasDossier = hasDossier
	}
	if attempt, ok := LatestAttempt(ledger, opts); ok {
		state.LatestAttempt = attempt
		state.HasAttempt = true
	}
	if model.Status == spec.StatusCompleted {
		state.Authority = TerminalAuthority(ledger)
		if state.Authority.ReviewEntry.Type == "review" {
			state.LatestReview = state.Authority.ReviewEntry
			state.HasReview = true
			if state.Authority.HasDossier {
				state.Dossier = state.Authority.Dossier
				state.HasDossier = true
			} else if dossier, ok := corereview.DecodeDossier(firstNonBlank(state.Authority.ReviewEntry.ReviewPacket, state.Authority.ReviewEntry.Output)); ok {
				state.Dossier = dossier
				state.HasDossier = true
			}
		}
		if state.Authority.Valid {
			state.Kind = KindCompletedAuthorized
			state.Reason = state.Authority.Reason
			state.Actual = state.Authority.Actual
			state.Next = "none"
			state.CanStartReview = false
			return state
		}
		state.Kind = KindInvalid
		state.Reason = state.Authority.Reason
		state.Actual = state.Authority.Actual
		state.Evidence = append([]string(nil), state.Authority.Evidence...)
		state.Next = "none"
		state.CanStartReview = false
		return state
	}
	if model.Status != spec.StatusReview {
		state.CanStartReview = false
		return state
	}
	if state.HasAttempt {
		switch state.LatestAttempt.Status {
		case AttemptStatusRunning:
			if state.LatestAttempt.Stale {
				state.Kind = KindAttemptStale
				state.Reason = "running review attempt lease expired"
				state.Actual = attemptActual(state.LatestAttempt)
				state.Evidence = []string{EntryReference(state.LatestAttempt.Entry)}
				state.ShouldAbandonAttempt = true
				state.CanStartReview = true
				state.Next = reviewCommand(taskID)
				return state
			}
			state.Kind = KindAttemptRunning
			state.Reason = "review attempt is already running"
			state.Actual = attemptActual(state.LatestAttempt)
			state.Evidence = []string{EntryReference(state.LatestAttempt.Entry)}
			state.CanStartReview = false
			state.Next = statusCommand(taskID)
			return state
		case AttemptStatusFailed:
			state.Kind = KindAttemptFailed
			state.Reason = firstNonBlank(state.LatestAttempt.Reason, "latest review attempt failed")
			state.Actual = state.Reason
			state.Evidence = attemptEvidence(state.LatestAttempt)
			state.CanStartReview = true
			state.Next = handoffCommand(taskID)
			return state
		case AttemptStatusAccepted:
			state.Kind = KindAttemptFailed
			state.Reason = "review attempt accepted but no review result was recorded"
			state.Actual = "latest review_attempt is accepted without a later review entry"
			state.Evidence = []string{EntryReference(state.LatestAttempt.Entry)}
			state.CanStartReview = true
			state.Next = reviewCommand(taskID)
			return state
		}
	}
	if latest, ok := latestGateEvent(ledger); ok {
		switch latest.Type {
		case "build", "criterion", "phase":
			if priorReviewExists(ledger) {
				state.Kind = KindReviewStaleAfterBuild
				state.Reason = "latest review is stale after newer build evidence"
				state.Actual = "latest gate event is " + EntryReference(latest)
				state.Evidence = []string{EntryReference(latest)}
				state.CanStartReview = true
				state.Next = reviewCommand(taskID)
				return state
			}
		}
	}
	if state.HasReview && state.LatestReview.Status == corereview.VerdictFail {
		state.Kind = KindReviewFailed
		state.Reason = "review verdict fail"
		state.Actual = "review verdict fail"
		state.Evidence = []string{EntryReference(state.LatestReview)}
		state.Blockers = reviewFindingSummaries(state.Dossier.Findings)
		state.CanStartReview = false
		state.ReviewBlockedUntilBuild = true
		state.Next = handoffCommand(taskID)
		return state
	}
	if state.Authority.Valid {
		if kind, reason, actual, stale := staleReviewAuthority(model, state.Authority, opts); stale {
			state.Kind = kind
			state.Reason = reason
			state.Actual = actual
			state.Evidence = []string{EntryReference(state.Authority.ReviewEntry)}
			state.CanStartReview = true
			state.Next = reviewCommand(taskID)
			return state
		}
		state.Kind = KindReviewPassed
		state.Reason = state.Authority.Reason
		state.Actual = state.Authority.Actual
		state.Evidence = append([]string(nil), state.Authority.Evidence...)
		state.CanStartReview = false
		state.Next = completeCommand(taskID)
		return state
	}
	if state.Authority.Found {
		state.Kind = KindInvalid
		state.Reason = state.Authority.Reason
		state.Actual = state.Authority.Actual
		state.Evidence = append([]string(nil), state.Authority.Evidence...)
		state.CanStartReview = true
		state.Next = reviewCommand(taskID)
		return state
	}
	state.Kind = KindReadyForReview
	state.Reason = "latest review gate has not passed"
	state.Actual = "no current accepted review"
	state.Evidence = append([]string(nil), state.Authority.Evidence...)
	state.CanStartReview = true
	state.Next = reviewCommand(taskID)
	return state
}

// LatestAttempt returns the newest review attempt in the current review window.
func LatestAttempt(ledger session.Session, opts Options) (Attempt, bool) {
	opts = normalizeOptions(opts)
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case EntryReviewAttempt:
			return DecodeAttempt(entry, opts), true
		case "review", "build", "criterion", "phase", "approval", session.EntryWorkspaceBaseline, "fail", "cancel", "complete":
			return Attempt{}, false
		}
	}
	return Attempt{}, false
}

// DecodeAttempt normalizes attempt metadata from a session entry.
func DecodeAttempt(entry session.Entry, opts Options) Attempt {
	opts = normalizeOptions(opts)
	attempt := Attempt{
		Entry:          entry,
		AttemptID:      firstNonBlank(entry.AttemptID, entry.ID),
		Status:         strings.TrimSpace(entry.Status),
		Provider:       strings.TrimSpace(entry.Provider),
		Model:          strings.TrimSpace(entry.ProviderModel),
		Mode:           corereview.Mode(strings.TrimSpace(entry.ReviewMode)),
		PassCount:      entry.ReviewPassCount,
		Reason:         strings.TrimSpace(entry.Reason),
		DiagnosticPath: strings.TrimSpace(entry.Path),
	}
	if attempt.Status == "" {
		attempt.Status = AttemptStatusRunning
	}
	attempt.Running = attempt.Status == AttemptStatusRunning
	if expires, ok := attemptLeaseExpiresAt(entry, opts); ok {
		attempt.LeaseExpiresAt = expires
		attempt.HasLease = true
		attempt.Stale = attempt.Running && !opts.Now.Before(expires)
	}
	return attempt
}

// NewAttemptID returns a deterministic attempt identifier for the task and start time.
func NewAttemptID(taskID string, now time.Time) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		taskID = "task"
	}
	return "review-" + taskID + "-" + strconv.FormatInt(now.UTC().UnixNano(), 10)
}

// RunningAttemptEntry builds the opening review_attempt entry.
func RunningAttemptEntry(input AttemptEntryInput) session.Entry {
	return session.Entry{
		Type:            EntryReviewAttempt,
		Status:          AttemptStatusRunning,
		Reason:          strings.TrimSpace(input.Reason),
		Provider:        strings.TrimSpace(input.Provider),
		Output:          strings.TrimSpace(input.Output),
		AttemptID:       strings.TrimSpace(input.AttemptID),
		LeaseExpiresAt:  formatTime(input.LeaseExpiresAt),
		ReviewMode:      string(input.Mode),
		ReviewPassCount: input.PassCount,
		ProviderModel:   strings.TrimSpace(input.Model),
	}
}

// AcceptedAttemptEntry explicitly closes a provider attempt that produced an
// accepted ReviewDossier. The following review entry carries the authority.
func AcceptedAttemptEntry(attempt Attempt, reason string) session.Entry {
	return closeAttemptEntry(attempt, AttemptStatusAccepted, reason, "")
}

// FailedAttemptEntry closes a provider attempt that did not produce an accepted dossier.
func FailedAttemptEntry(attempt Attempt, reason string, diagnosticPath string) session.Entry {
	return closeAttemptEntry(attempt, AttemptStatusFailed, reason, diagnosticPath)
}

// AbandonedAttemptEntry closes a stale running attempt before a new attempt starts.
func AbandonedAttemptEntry(attempt Attempt, reason string) session.Entry {
	return closeAttemptEntry(attempt, AttemptStatusAbandoned, reason, attempt.DiagnosticPath)
}

func closeAttemptEntry(attempt Attempt, status string, reason string, diagnosticPath string) session.Entry {
	return session.Entry{
		Type:            EntryReviewAttempt,
		Status:          status,
		Reason:          strings.TrimSpace(reason),
		Provider:        attempt.Provider,
		ProviderModel:   attempt.Model,
		Output:          strings.TrimSpace(attempt.Entry.Output),
		Path:            strings.TrimSpace(diagnosticPath),
		AttemptID:       attempt.AttemptID,
		LeaseExpiresAt:  formatTime(attempt.LeaseExpiresAt),
		ReviewMode:      string(attempt.Mode),
		ReviewPassCount: attempt.PassCount,
	}
}

func staleReviewAuthority(model spec.Model, authority Authority, opts Options) (Kind, string, string, bool) {
	if !authority.Valid || authority.HumanReviewed || authority.ReviewEntry.Type != "review" {
		return "", "", "", false
	}
	recordedSpec := strings.TrimSpace(authority.ReviewEntry.ReviewedSpec)
	if recordedSpec == "" {
		return KindReviewStaleAfterSpec, "latest review is stale against current spec", "reviewed_spec is missing", true
	}
	currentSpec := spec.ContractDigest(model)
	if recordedSpec != currentSpec {
		return KindReviewStaleAfterSpec, "latest review is stale against current spec", fmt.Sprintf("reviewed_spec %s, current %s", recordedSpec, currentSpec), true
	}
	if strings.TrimSpace(authority.ReviewEntry.ReviewedMaterialDigest) != "" {
		if len(authority.ReviewEntry.ReviewedScope) == 0 {
			return KindReviewStaleAfterWorkspace, "latest review material seal is incomplete", "reviewed_scope is missing", true
		}
		if !opts.HasMaterialSeal {
			return "", "", "", false
		}
		recordedMaterial := strings.TrimSpace(authority.ReviewEntry.ReviewedMaterialDigest)
		currentMaterial := strings.TrimSpace(opts.MaterialSeal.Digest)
		if recordedMaterial != currentMaterial {
			return KindReviewStaleAfterWorkspace, "latest review is stale against current task material", fmt.Sprintf("reviewed_material_digest %s, current %s", recordedMaterial, currentMaterial), true
		}
		return "", "", "", false
	}
	if !opts.HasWorkspaceSeal {
		return "", "", "", false
	}
	recorded := WorkspaceSeal{
		Head:  strings.TrimSpace(authority.ReviewEntry.ReviewedHead),
		Dirty: strings.TrimSpace(authority.ReviewEntry.ReviewedDirty),
		Diff:  strings.TrimSpace(authority.ReviewEntry.ReviewedDiff),
	}
	current := WorkspaceSeal{
		Head:  strings.TrimSpace(opts.WorkspaceSeal.Head),
		Dirty: strings.TrimSpace(opts.WorkspaceSeal.Dirty),
		Diff:  strings.TrimSpace(opts.WorkspaceSeal.Diff),
	}
	if recorded.Head != current.Head {
		return KindReviewStaleAfterWorkspace, "latest review is stale against current workspace", fmt.Sprintf("reviewed_head %s, current %s", recorded.Head, current.Head), true
	}
	if recorded.Dirty != current.Dirty {
		return KindReviewStaleAfterWorkspace, "latest review is stale against current workspace", fmt.Sprintf("reviewed_dirty %s, current %s", recorded.Dirty, current.Dirty), true
	}
	if recorded.Diff != current.Diff {
		return KindReviewStaleAfterWorkspace, "latest review is stale against current workspace", fmt.Sprintf("reviewed_diff %s, current %s", recorded.Diff, current.Diff), true
	}
	return "", "", "", false
}

func latestVisibleReview(ledger session.Session) (session.Entry, corereview.Dossier, bool, bool) {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "review":
			dossier, ok := corereview.DecodeDossier(firstNonBlank(entry.ReviewPacket, entry.Output))
			return entry, dossier, true, ok
		case EntryReviewAttempt:
			continue
		case "build", "criterion", "phase", "approval", session.EntryWorkspaceBaseline, "fail", "cancel", "complete":
			return session.Entry{}, corereview.Dossier{}, false, false
		}
	}
	return session.Entry{}, corereview.Dossier{}, false, false
}

func latestGateEvent(ledger session.Session) (session.Entry, bool) {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case session.EntryReceipt, "review", EntryReviewAttempt, "build", "criterion", "phase", "approval", session.EntryWorkspaceBaseline, "fail", "cancel", "complete":
			return entry, true
		}
	}
	return session.Entry{}, false
}

func priorReviewExists(ledger session.Session) bool {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "review":
			return true
		case "approval", session.EntryWorkspaceBaseline, "fail", "cancel", "complete":
			return false
		}
	}
	return false
}

func attemptLeaseExpiresAt(entry session.Entry, opts Options) (time.Time, bool) {
	if expires, ok := parseTime(entry.LeaseExpiresAt); ok {
		return expires, true
	}
	recorded, ok := parseTime(entry.RecordedAt)
	if !ok {
		return time.Time{}, false
	}
	return recorded.Add(opts.AttemptLease), true
}

func normalizeOptions(opts Options) Options {
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	opts.Now = opts.Now.UTC()
	if opts.AttemptLease <= 0 {
		opts.AttemptLease = DefaultAttemptLease
	}
	return opts
}

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func attemptActual(attempt Attempt) string {
	if attempt.HasLease && !attempt.LeaseExpiresAt.IsZero() {
		return fmt.Sprintf("review_attempt %s lease expires at %s", attempt.Status, formatTime(attempt.LeaseExpiresAt))
	}
	return "review_attempt " + attempt.Status
}

func attemptEvidence(attempt Attempt) []string {
	if attempt.DiagnosticPath != "" {
		return []string{attempt.DiagnosticPath}
	}
	return []string{EntryReference(attempt.Entry)}
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

func reviewCommand(taskID string) string {
	if taskID == "" {
		return "scafld review"
	}
	return "scafld review " + taskID
}

func handoffCommand(taskID string) string {
	if taskID == "" {
		return "scafld handoff"
	}
	return "scafld handoff " + taskID
}

func completeCommand(taskID string) string {
	if taskID == "" {
		return "scafld complete"
	}
	return "scafld complete " + taskID
}

func statusCommand(taskID string) string {
	if taskID == "" {
		return "scafld status"
	}
	return "scafld status " + taskID
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
