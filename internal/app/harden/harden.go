package harden

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/app/specsource"
	"github.com/nilstate/scafld/v2/internal/core/agentcontract"
	"github.com/nilstate/scafld/v2/internal/core/diagnostics"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
	"github.com/nilstate/scafld/v2/internal/core/hardengate"
	"github.com/nilstate/scafld/v2/internal/core/lifecycle"
	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

var (
	// ErrMissingSpecStore is returned when hardening has no spec store.
	ErrMissingSpecStore = errors.New("missing spec store")
	// ErrNoHardenRound is returned when marking pass without an open round.
	ErrNoHardenRound = errors.New("no harden round exists")
	// ErrSpecNotDraft is returned when hardening a non-draft spec.
	ErrSpecNotDraft = errors.New("harden only operates on drafts")
	// ErrInvalidHardenEvidence is returned when a hardening round has unverified citations.
	ErrInvalidHardenEvidence = errors.New("invalid harden evidence")
	// ErrHardenRoundOpen is returned when another harden round is still open.
	ErrHardenRoundOpen = errors.New("harden round already in progress")
	// ErrHardenRevisionRequired is returned when a provider harden rerun is attempted without draft edits.
	ErrHardenRevisionRequired = errors.New("harden revision required before rerun")
)

const anchorShape = `expected "Anchor: spec_gap:<field>", "Anchor: code:<path>:<line>", or "Anchor: archive:<task-id>"`

var requiredHardenDimensions = append([]string(nil), coreharden.RequiredDimensions...)

// SpecStore is the spec persistence port used by hardening.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

// Clock supplies hardening timestamps.
type Clock interface {
	Now() time.Time
}

// Provider is the external hardening provider port.
type Provider interface {
	Invoke(context.Context, coreharden.Request) (coreharden.Dossier, error)
}

// Input describes a hardening operation.
type Input struct {
	TaskID                  string
	MarkPassed              bool
	Root                    string
	Prompt                  string
	Contract                agentcontract.Contract
	Provider                Provider
	ContextMaxBytes         int
	RequiredContextMaxBytes int
	SuppressContext         bool
}

// Output describes the opened or completed hardening round.
type Output struct {
	TaskID       string            `json:"task_id"`
	Path         string            `json:"path"`
	HardenStatus spec.HardenStatus `json:"harden_status"`
	RoundID      string            `json:"round_id"`
	MarkedPassed bool              `json:"marked_passed"`
	NextCommand  string            `json:"next_command"`
	Prompt       string            `json:"prompt"`
	Context      string            `json:"context,omitempty"`
	Verdict      string            `json:"verdict,omitempty"`
	Summary      string            `json:"summary,omitempty"`
	Provider     string            `json:"provider,omitempty"`
	Model        string            `json:"model,omitempty"`
	SpecDigest   string            `json:"spec_digest,omitempty"`
	Warnings     []string          `json:"warnings"`
}

// Run opens a hardening round or marks the latest round passed.
func Run(ctx context.Context, store SpecStore, clock Clock, input Input) (Output, error) {
	if store == nil {
		return Output{}, ErrMissingSpecStore
	}
	if clock == nil {
		clock = systemClock{}
	}
	source, err := specsource.Load(ctx, store, input.TaskID)
	if err != nil {
		return Output{}, err
	}
	model := source.Model
	path := source.Path
	if !specPathIsDraft(path) {
		return Output{}, fmt.Errorf("%w: %s", ErrSpecNotDraft, path)
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	if input.MarkPassed {
		return markPassed(ctx, store, path, model, now, input.Root)
	}
	specDigest := spec.HardenContractDigest(model)
	state := hardengate.Project(model)
	if out, err, handled := guardExistingHardenRound(ctx, store, path, model, input, state); handled {
		return out, err
	}
	if input.Provider != nil {
		return runProviderHarden(ctx, store, input.Provider, path, model, now, input.Root, input.Contract, input.ContextMaxBytes, input.RequiredContextMaxBytes, specDigest)
	}
	return openRound(ctx, store, path, model, now, input.Contract, input.Prompt, input.ContextMaxBytes, input.RequiredContextMaxBytes, input.SuppressContext, specDigest)
}

func guardExistingHardenRound(ctx context.Context, store SpecStore, path string, model spec.Model, input Input, state hardengate.State) (Output, error, bool) {
	if !state.HasRound {
		return Output{}, nil, false
	}
	round := state.LatestRound
	if state.Kind == hardengate.KindRoundOpen {
		if input.Provider != nil {
			out := Output{TaskID: model.TaskID, Path: path, HardenStatus: spec.HardenInProgress, RoundID: round.ID, NextCommand: hardengate.MarkPassedCommand(model.TaskID), SpecDigest: round.SpecDigest}
			return out, gate.New(ErrHardenRoundOpen, gate.Failure{
				Gate:     "harden",
				Status:   string(model.Status),
				Reason:   "a harden round is already in progress",
				Expected: "finish the current harden round before starting another provider pass",
				Actual:   round.ID + " is in_progress",
				Blockers: []string{"current harden round is still in_progress"},
				Next:     hardengate.MarkPassedCommand(model.TaskID),
			}), true
		}
		out, err := existingOpenRoundOutput(ctx, store, path, model, round, input.Contract, input.Prompt, input.ContextMaxBytes, input.RequiredContextMaxBytes, input.SuppressContext)
		return out, err, true
	}
	switch state.Kind {
	case hardengate.KindPassed:
		return Output{
			TaskID:       model.TaskID,
			Path:         path,
			HardenStatus: spec.HardenPassed,
			RoundID:      round.ID,
			NextCommand:  hardengate.ApproveCommand(model.TaskID),
			Verdict:      coreharden.VerdictPass,
			Summary:      "hardening already passed for this draft revision",
			Provider:     round.Provider,
			Model:        round.Model,
			SpecDigest:   state.CurrentDigest,
		}, nil, true
	case hardengate.KindNeedsOperatorDecision:
		if !state.ProviderRerunBlocked {
			return Output{}, nil, false
		}
		next := hardengate.NeedsRevisionFollowUp(model.TaskID, input.Provider != nil)
		out := Output{TaskID: model.TaskID, Path: path, HardenStatus: spec.HardenNeedsRevision, RoundID: round.ID, NextCommand: next, Verdict: coreharden.VerdictNeedsRevision, Summary: "latest harden round already covers this draft revision", Provider: round.Provider, Model: round.Model, SpecDigest: state.CurrentDigest}
		return out, gate.New(ErrHardenRevisionRequired, gate.Failure{
			Gate:     "harden",
			Status:   string(model.Status),
			Reason:   "latest harden round already covers this draft revision",
			Evidence: state.Evidence,
			Expected: "edit the draft contract before rerunning harden, or approve if the operator rejects the finding as bookkeeping",
			Actual:   "draft spec digest unchanged since " + round.ID,
			Blockers: state.Blockers,
			Next:     next,
		}), true
	default:
		return Output{}, nil, false
	}
}

func existingOpenRoundOutput(ctx context.Context, store SpecStore, path string, model spec.Model, round spec.HardenRound, contract agentcontract.Contract, prompt string, contextMaxBytes int, requiredContextMaxBytes int, suppressContext bool) (Output, error) {
	renderedContext := ""
	if !suppressContext {
		source, err := specsource.Load(ctx, store, model.TaskID)
		if err != nil {
			return Output{}, err
		}
		packet := hardenContextPacket(source, contract, manualHardenOutputSection(hardengate.MarkPassedCommand(model.TaskID)))
		renderedContext = reviewcontext.RenderMarkdown(packet, reviewcontext.Options{MaxBytes: contextMaxBytes, RequiredMaxBytes: requiredContextMaxBytes, Title: "Harden Context Packet"})
	}
	return Output{
		TaskID:       model.TaskID,
		Path:         path,
		HardenStatus: spec.HardenInProgress,
		RoundID:      round.ID,
		NextCommand:  hardengate.MarkPassedCommand(model.TaskID),
		Prompt:       prompt,
		Context:      renderedContext,
		SpecDigest:   round.SpecDigest,
	}, nil
}

func runProviderHarden(ctx context.Context, store SpecStore, provider Provider, path string, model spec.Model, now string, root string, contract agentcontract.Contract, contextMaxBytes int, requiredContextMaxBytes int, specDigest string) (Output, error) {
	roundID := nextRoundID(model.HardenRounds)
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = append(model.HardenRounds, spec.HardenRound{
		ID:         roundID,
		Status:     string(spec.HardenInProgress),
		StartedAt:  now,
		SpecDigest: specDigest,
	})
	model.Updated = now
	model.CurrentState.Next = "harden"
	model.CurrentState.Reason = "external hardening provider running"
	model.CurrentState.Blockers = "provider hardening not yet recorded"
	model.CurrentState.AllowedFollowUp = "scafld harden " + model.TaskID + " --provider <provider>"
	if err := store.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save harden round: %w", err)
	}
	source, err := specsource.Load(ctx, store, model.TaskID)
	if err != nil {
		reason, diagnosticPath := diagnostics.FailureReason("provider harden source reload failed", err, 240)
		if closeErr := closeProviderHardenRound(ctx, store, model.TaskID, roundID, now, reason, diagnosticPath, "external hardening setup error", "fix local spec access, then run scafld harden "+model.TaskID+" --provider <provider>"); closeErr != nil {
			return Output{}, errors.Join(err, fmt.Errorf("record provider harden failure: %w", closeErr))
		}
		return Output{}, err
	}
	model = source.Model
	packet := hardenContextPacket(source, contract, providerHardenOutputSection())
	rendered, err := reviewcontext.RenderMarkdownStrict(packet, reviewcontext.Options{MaxBytes: contextMaxBytes, RequiredMaxBytes: requiredContextMaxBytes, Title: "Harden Context Packet"})
	if err != nil {
		reason, diagnosticPath := diagnostics.FailureReason("provider harden context invalid", err, 240)
		if closeErr := closeProviderHardenFailure(ctx, store, model.TaskID, roundID, now, reason, diagnosticPath); closeErr != nil {
			return Output{}, errors.Join(err, fmt.Errorf("record provider harden failure: %w", closeErr))
		}
		return Output{}, err
	}
	dossier, err := provider.Invoke(ctx, coreharden.Request{TaskID: model.TaskID, Prompt: rendered, Context: packet})
	if err != nil {
		reason, diagnosticPath := diagnostics.FailureReason("provider error", err, 240)
		if closeErr := closeProviderHardenFailure(ctx, store, model.TaskID, roundID, now, reason, diagnosticPath); closeErr != nil {
			return Output{}, errors.Join(err, fmt.Errorf("record provider harden failure: %w", closeErr))
		}
		return Output{}, err
	}
	if err := coreharden.ValidateDossier(dossier); err != nil {
		reason, diagnosticPath := diagnostics.FailureReason("invalid provider dossier", err, 240)
		if closeErr := closeProviderHardenFailure(ctx, store, model.TaskID, roundID, now, reason, diagnosticPath); closeErr != nil {
			return Output{}, errors.Join(err, fmt.Errorf("record provider harden failure: %w", closeErr))
		}
		return Output{}, err
	}
	recordCtx, cancelRecord := lifecycle.TerminalEvidenceContext(ctx)
	defer cancelRecord()
	taskID := model.TaskID
	reloaded, _, err := store.Load(recordCtx, taskID)
	if err != nil {
		if closeErr := closeProviderHardenTerminalRecordFailure(ctx, store, taskID, roundID, now, "load harden round after provider", err); closeErr != nil {
			return Output{}, closeErr
		}
		return Output{}, err
	}
	model = reloaded
	if len(model.HardenRounds) == 0 || model.HardenRounds[len(model.HardenRounds)-1].ID != roundID {
		return Output{}, fmt.Errorf("harden round changed while provider was running")
	}
	latest := len(model.HardenRounds) - 1
	model.HardenRounds[latest] = roundFromDossier(model.HardenRounds[latest], dossier, now)
	model.Updated = now
	if warnings := verifyHardenRoundShape(root, model, model.HardenRounds[latest], true); len(warnings) > 0 {
		reason := "invalid provider dossier evidence: " + strings.Join(warnings, "; ")
		model.HardenRounds[latest].Status = string(spec.HardenError)
		model.HardenRounds[latest].Summary = reason
		model.HardenStatus = spec.HardenError
		model.CurrentState.Next = "harden"
		model.CurrentState.Reason = "external hardening provider evidence error"
		model.CurrentState.Blockers = reason
		model.CurrentState.AllowedFollowUp = "fix provider harden evidence, then run scafld harden " + model.TaskID + " --provider <provider>"
		if err := store.Save(recordCtx, path, model); err != nil {
			return Output{}, closeProviderHardenTerminalRecordFailure(ctx, store, model.TaskID, roundID, now, "save harden evidence failure", err)
		}
		out := Output{TaskID: model.TaskID, Path: path, HardenStatus: model.HardenStatus, RoundID: roundID, Warnings: warnings, Summary: reason, Provider: dossier.Provider, Model: dossier.Model}
		return out, gate.New(ErrInvalidHardenEvidence, gate.Failure{
			Gate:     "harden",
			Status:   string(model.Status),
			Reason:   "provider hardening evidence is invalid",
			Evidence: warnings,
			Expected: "observations with filesystem-verifiable anchors",
			Actual:   strings.Join(warnings, "; "),
			Blockers: warnings,
			Next:     "fix provider harden evidence, then run scafld harden " + model.TaskID + " --provider <provider>",
		})
	}
	verdict := coreharden.VerdictFromDossier(dossier)
	model.HardenRounds[latest].Verdict = verdict
	if verdict == coreharden.VerdictPass {
		model.HardenStatus = spec.HardenPassed
		model.HardenRounds[latest].Status = string(spec.HardenPassed)
		model.CurrentState.Next = "approve"
		model.CurrentState.Reason = "hardening passed"
		model.CurrentState.Blockers = "none"
		model.CurrentState.AllowedFollowUp = "scafld approve " + model.TaskID
	} else {
		model.HardenStatus = spec.HardenNeedsRevision
		model.HardenRounds[latest].Status = string(spec.HardenNeedsRevision)
		model.CurrentState.Next = "operator_decision"
		model.CurrentState.Reason = "hardening found draft shape findings requiring operator judgment"
		model.CurrentState.Blockers = hardenBlockers(dossier)
		model.CurrentState.AllowedFollowUp = hardengate.NeedsRevisionFollowUp(model.TaskID, true)
	}
	if err := store.Save(recordCtx, path, model); err != nil {
		return Output{}, closeProviderHardenTerminalRecordFailure(ctx, store, model.TaskID, roundID, now, "save harden dossier", err)
	}
	return Output{
		TaskID:       model.TaskID,
		Path:         path,
		HardenStatus: model.HardenStatus,
		RoundID:      roundID,
		NextCommand:  model.CurrentState.AllowedFollowUp,
		Verdict:      verdict,
		Summary:      dossier.Summary,
		Provider:     dossier.Provider,
		Model:        dossier.Model,
		SpecDigest:   specDigest,
	}, nil
}

func closeProviderHardenFailure(ctx context.Context, store SpecStore, taskID string, roundID string, now string, reason string, diagnosticPath string) error {
	return closeProviderHardenRound(ctx, store, taskID, roundID, now, reason, diagnosticPath, "external hardening provider error", "fix provider availability/output, then run scafld harden "+taskID+" --provider <provider>")
}

func closeProviderHardenRound(ctx context.Context, store SpecStore, taskID string, roundID string, now string, reason string, diagnosticPath string, stateReason string, followUp string) error {
	writeCtx, cancel := lifecycle.TerminalEvidenceContext(ctx)
	defer cancel()
	model, path, err := store.Load(writeCtx, taskID)
	if err != nil {
		return err
	}
	if len(model.HardenRounds) == 0 || model.HardenRounds[len(model.HardenRounds)-1].ID != roundID {
		return fmt.Errorf("harden round changed while provider was running")
	}
	latest := len(model.HardenRounds) - 1
	model.HardenRounds[latest].Status = string(spec.HardenError)
	model.HardenRounds[latest].EndedAt = now
	model.HardenRounds[latest].Summary = reason
	model.HardenRounds[latest].DiagnosticPath = diagnosticPath
	model.HardenStatus = spec.HardenError
	model.Updated = now
	model.CurrentState.Next = "harden"
	model.CurrentState.Reason = stateReason
	model.CurrentState.Blockers = reason
	model.CurrentState.AllowedFollowUp = followUp
	return store.Save(writeCtx, path, model)
}

func closeProviderHardenTerminalRecordFailure(ctx context.Context, store SpecStore, taskID string, roundID string, now string, label string, cause error) error {
	recordErr := fmt.Errorf("%s: %w", label, cause)
	reason, diagnosticPath := diagnostics.FailureReason("provider harden terminal recording failed", recordErr, 240)
	if closeErr := closeProviderHardenRound(ctx, store, taskID, roundID, now, reason, diagnosticPath, "external hardening terminal recording error", "fix local spec storage, then run scafld harden "+taskID+" --provider <provider>"); closeErr != nil {
		return errors.Join(recordErr, fmt.Errorf("record provider harden failure: %w", closeErr))
	}
	return recordErr
}

func openRound(ctx context.Context, store SpecStore, path string, model spec.Model, now string, contract agentcontract.Contract, prompt string, contextMaxBytes int, requiredContextMaxBytes int, suppressContext bool, specDigest string) (Output, error) {
	roundID := nextRoundID(model.HardenRounds)
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = append(model.HardenRounds, spec.HardenRound{
		ID:           roundID,
		Status:       string(spec.HardenInProgress),
		StartedAt:    now,
		SpecDigest:   specDigest,
		Observations: hardenObservationSkeleton(),
	})
	model.Updated = now
	model.CurrentState.Next = "harden"
	model.CurrentState.Reason = "hardening round in progress"
	model.CurrentState.Blockers = manualHardenBlocker()
	model.CurrentState.AllowedFollowUp = manualHardenFollowUp(path, model.TaskID)
	if err := store.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save harden round: %w", err)
	}
	renderedContext := ""
	if !suppressContext {
		source, err := specsource.Load(ctx, store, model.TaskID)
		if err != nil {
			return Output{}, err
		}
		packet := hardenContextPacket(source, contract, manualHardenOutputSection(hardengate.MarkPassedCommand(model.TaskID)))
		renderedContext = reviewcontext.RenderMarkdown(packet, reviewcontext.Options{MaxBytes: contextMaxBytes, RequiredMaxBytes: requiredContextMaxBytes, Title: "Harden Context Packet"})
	}
	return Output{
		TaskID:       model.TaskID,
		Path:         path,
		HardenStatus: model.HardenStatus,
		RoundID:      roundID,
		NextCommand:  hardengate.MarkPassedCommand(model.TaskID),
		Prompt:       prompt,
		Context:      renderedContext,
		SpecDigest:   specDigest,
	}, nil
}

func hardenObservationSkeleton() []spec.HardenObservation {
	observations := make([]spec.HardenObservation, 0, len(requiredHardenDimensions))
	for _, dimension := range requiredHardenDimensions {
		observations = append(observations, spec.HardenObservation{Dimension: dimension})
	}
	return observations
}

func manualHardenBlocker() string {
	return "fill harden observations with Result and Anchor before marking passed"
}

func manualHardenFollowUp(path string, taskID string) string {
	return "fill harden observations in " + path + ", then run " + hardengate.MarkPassedCommand(taskID)
}

func markPassed(ctx context.Context, store SpecStore, path string, model spec.Model, now string, root string) (Output, error) {
	if len(model.HardenRounds) == 0 {
		return Output{}, ErrNoHardenRound
	}
	latest := len(model.HardenRounds) - 1
	warnings := verifyHardenRoundEvidence(root, model, model.HardenRounds[latest])
	if len(warnings) > 0 {
		summary := hardenEvidenceSummary(warnings)
		out := Output{TaskID: model.TaskID, Path: path, HardenStatus: model.HardenStatus, RoundID: model.HardenRounds[latest].ID, Warnings: warnings}
		return out, gate.New(ErrInvalidHardenEvidence, gate.Failure{
			Gate:     "harden",
			Status:   string(model.Status),
			Reason:   "hardening evidence is incomplete",
			Evidence: warnings,
			Expected: "required harden observations with verified anchors and no unresolved blocking observations",
			Actual:   summary,
			Blockers: hardenEvidenceBlockers(warnings),
			Next:     "fix the harden observations, then run scafld harden " + model.TaskID + " --mark-passed",
		})
	}
	model.HardenRounds[latest].Status = string(spec.HardenPassed)
	model.HardenRounds[latest].EndedAt = now
	model.HardenStatus = spec.HardenPassed
	model.Updated = now
	model.CurrentState.Next = "approve"
	model.CurrentState.Reason = "hardening passed"
	model.CurrentState.Blockers = "none"
	model.CurrentState.AllowedFollowUp = "scafld approve " + model.TaskID
	if err := store.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save harden pass: %w", err)
	}
	return Output{
		TaskID:       model.TaskID,
		Path:         path,
		HardenStatus: model.HardenStatus,
		RoundID:      model.HardenRounds[latest].ID,
		MarkedPassed: true,
		NextCommand:  model.CurrentState.AllowedFollowUp,
		Warnings:     warnings,
	}, nil
}

func hardenEvidenceSummary(warnings []string) string {
	return fmt.Sprintf("%d harden evidence issue(s): every required observation needs a valid Result and Anchor before --mark-passed", len(warnings))
}

func hardenEvidenceBlockers(warnings []string) []string {
	blockers := make([]string, 0, len(warnings)+1)
	blockers = append(blockers, hardenEvidenceSummary(warnings))
	blockers = append(blockers, warnings...)
	return blockers
}

func roundFromDossier(round spec.HardenRound, dossier coreharden.Dossier, now string) spec.HardenRound {
	round.EndedAt = now
	round.Verdict = coreharden.VerdictFromDossier(dossier)
	round.Summary = dossier.Summary
	round.Provider = dossier.Provider
	round.Model = dossier.Model
	round.OutputFormat = dossier.OutputFormat
	round.Shape = spec.HardenShape{
		Decision:          dossier.Shape.Decision,
		TrueShape:         dossier.Shape.TrueShape,
		MinimalPlan:       dossier.Shape.MinimalPlan,
		SharedOwner:       dossier.Shape.SharedOwner,
		AdapterBoundaries: append([]string(nil), dossier.Shape.AdapterBoundaries...),
		RequiredSpecEdits: append([]string(nil), dossier.Shape.RequiredSpecEdits...),
	}
	round.Observations = make([]spec.HardenObservation, 0, len(dossier.Observations))
	for _, observation := range dossier.Observations {
		round.Observations = append(round.Observations, spec.HardenObservation{
			Dimension:    observation.Dimension,
			Result:       observation.Result,
			Anchor:       observation.Anchor,
			Note:         observation.Note,
			Question:     observation.Question,
			Recommended:  observation.Recommended,
			IfUnanswered: observation.IfUnanswered,
			Default:      observation.Default,
			Status:       observation.Status,
		})
	}
	return round
}

func hardenBlockers(dossier coreharden.Dossier) string {
	var blockers []string
	if coreharden.ShapeRequiresRevision(dossier.Shape) {
		if dossier.Shape.Decision != coreharden.DecisionKeep {
			blockers = append(blockers, "shape decision requires revision: "+dossier.Shape.Decision)
		}
		if len(dossier.Shape.RequiredSpecEdits) > 0 {
			blockers = append(blockers, fmt.Sprintf("%d required spec edit(s)", len(dossier.Shape.RequiredSpecEdits)))
		}
	}
	for _, missing := range coreharden.MissingDimensions(dossier.Observations) {
		blockers = append(blockers, "missing observation: "+missing)
	}
	openBlocks := 0
	for _, observation := range dossier.Observations {
		if coreharden.ObservationBlocksApproval(observation) {
			openBlocks++
		}
	}
	if openBlocks > 0 {
		blockers = append(blockers, fmt.Sprintf("%d unresolved blocking observation(s)", openBlocks))
	}
	if len(blockers) == 0 {
		return "harden observations need revision"
	}
	return strings.Join(blockers, "; ")
}

func nextRoundID(rounds []spec.HardenRound) string {
	seen := map[string]bool{}
	for _, round := range rounds {
		seen[round.ID] = true
	}
	for i := len(rounds) + 1; ; i++ {
		id := fmt.Sprintf("round-%d", i)
		if !seen[id] {
			return id
		}
	}
}

func verifyHardenRoundEvidence(root string, model spec.Model, round spec.HardenRound) []string {
	return verifyHardenRoundShape(root, model, round, false)
}

func verifyHardenRoundShape(root string, model spec.Model, round spec.HardenRound, allowOpenBlocks bool) []string {
	if root == "" {
		root = "."
	}
	var warnings []string
	warnings = append(warnings, verifyHardenShapeDecision(round.Shape, allowOpenBlocks)...)
	warnings = append(warnings, verifyHardenObservations(root, model, round.Observations, allowOpenBlocks)...)
	return warnings
}

func verifyHardenShapeDecision(shape spec.HardenShape, allowRevision bool) []string {
	coreShape := coreharden.Shape{
		Decision:          shape.Decision,
		TrueShape:         shape.TrueShape,
		MinimalPlan:       shape.MinimalPlan,
		SharedOwner:       shape.SharedOwner,
		AdapterBoundaries: append([]string(nil), shape.AdapterBoundaries...),
		RequiredSpecEdits: append([]string(nil), shape.RequiredSpecEdits...),
	}
	var warnings []string
	if err := coreharden.ValidateShape(coreShape); err != nil {
		warnings = append(warnings, err.Error())
		return warnings
	}
	if !allowRevision && coreharden.ShapeRequiresRevision(coreShape) {
		if coreShape.Decision != coreharden.DecisionKeep {
			warnings = append(warnings, "shape decision requires spec revision before harden can pass: "+coreShape.Decision)
		}
		if len(coreShape.RequiredSpecEdits) > 0 {
			warnings = append(warnings, fmt.Sprintf("shape has %d required spec edit(s) before harden can pass", len(coreShape.RequiredSpecEdits)))
		}
	}
	return warnings
}

func verifyHardenObservations(root string, model spec.Model, observations []spec.HardenObservation, allowOpenBlocks bool) []string {
	var warnings []string
	if len(observations) == 0 {
		return []string{"missing harden observations: record " + coreharden.RequiredDimensionList() + " observations before marking hardening passed"}
	}
	seen := map[string]bool{}
	for i, observation := range observations {
		idx := i + 1
		dimension := normalizeDimension(observation.Dimension)
		label := fmt.Sprintf("observation %d", idx)
		if dimension != "" {
			label = fmt.Sprintf("observation %q", dimension)
		}
		if dimension == "" {
			warnings = append(warnings, fmt.Sprintf("%s: missing dimension", label))
		} else if !coreharden.ValidDimension(dimension) {
			warnings = append(warnings, fmt.Sprintf("%s: invalid dimension %q", label, observation.Dimension))
		} else {
			seen[dimension] = true
		}
		result := normalizeResult(observation.Result)
		switch result {
		case coreharden.ResultClean, coreharden.ResultNotApplicable:
		case coreharden.ResultAdvisory:
			if strings.TrimSpace(observation.Note) == "" {
				warnings = append(warnings, fmt.Sprintf("%s: advisory result requires note", label))
			}
		case coreharden.ResultBlocks:
			if strings.TrimSpace(observation.Note) == "" {
				warnings = append(warnings, fmt.Sprintf("%s: blocks result requires note", label))
			}
			if !allowOpenBlocks && observationOpen(observation.Status) {
				warnings = append(warnings, fmt.Sprintf("%s: blocking observation is still open", label))
			}
		default:
			warnings = append(warnings, fmt.Sprintf("%s: result must be clean, advisory, blocks, or n/a before marking hardening passed", label))
		}
		if strings.TrimSpace(observation.Status) != "" && !coreharden.ValidObservationStatus(observation.Status) {
			warnings = append(warnings, fmt.Sprintf("%s: invalid status %q", label, observation.Status))
		}
		warnings = append(warnings, verifyAnchor(root, model, label, anchorShape, observation.Anchor)...)
	}
	var missing []string
	for _, required := range requiredHardenDimensions {
		if !seen[required] {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		warnings = append(warnings, fmt.Sprintf("missing required harden observations: %s (record each under Observations with Result, Anchor, and optional Note/Default/Status)", strings.Join(missing, ", ")))
	}
	return warnings
}

func observationOpen(status string) bool {
	switch normalizeDimension(status) {
	case "", coreharden.StatusOpen:
		return true
	case coreharden.StatusFixed, coreharden.StatusAcceptedRisk, coreharden.StatusSuperseded:
		return false
	default:
		return true
	}
}

func verifyAnchor(root string, model spec.Model, label string, expected string, anchor string) []string {
	anchor = strings.TrimSpace(anchor)
	switch {
	case anchor == "":
		return []string{fmt.Sprintf("%s: missing anchor (%s)", label, expected)}
	case strings.HasPrefix(anchor, "spec_gap:"):
		return verifySpecGapCitation(model, label, anchor)
	case strings.HasPrefix(anchor, "code:"):
		return verifyCodeCitation(root, label, anchor)
	case strings.HasPrefix(anchor, "archive:"):
		return verifyArchiveCitation(root, label, anchor)
	default:
		return []string{fmt.Sprintf("%s: invalid anchor prefix %q (%s)", label, anchor, expected)}
	}
}

func verifySpecGapCitation(model spec.Model, label string, anchor string) []string {
	field := strings.TrimSpace(strings.TrimPrefix(anchor, "spec_gap:"))
	if field == "" {
		return []string{fmt.Sprintf("%s: spec_gap citation is empty (expected spec_gap:<field>)", label)}
	}
	if validSpecGapField(model, field) {
		return nil
	}
	return []string{fmt.Sprintf("%s: spec_gap citation does not name a known spec field: %s", label, anchor)}
}

func validSpecGapField(model spec.Model, field string) bool {
	key := normalizeSpecGapField(field)
	known := map[string]bool{
		"acceptance":   true,
		"assumptions":  true,
		"context":      true,
		"currentstate": true,
		"dependencies": true,
		"deviations":   true,
		"hardening":    true,
		"hardenrounds": true,
		"metadata":     true,
		"objectives":   true,
		"origin":       true,
		"phases":       true,
		"planninglog":  true,
		"review":       true,
		"risks":        true,
		"rollback":     true,
		"scope":        true,
		"selfeval":     true,
		"summary":      true,
		"touchpoints":  true,
		"validation":   true,
	}
	if known[key] {
		return true
	}
	for _, phase := range model.Phases {
		if key == normalizeSpecGapField(phase.ID) || key == normalizeSpecGapField(phase.Name) {
			return true
		}
	}
	for _, criterion := range model.Acceptance.Criteria {
		if key == normalizeSpecGapField(criterion.ID) || key == normalizeSpecGapField(criterion.Title) {
			return true
		}
	}
	for _, phase := range model.Phases {
		for _, criterion := range phase.Acceptance {
			if key == normalizeSpecGapField(criterion.ID) || key == normalizeSpecGapField(criterion.Title) {
				return true
			}
		}
	}
	return false
}

func normalizeSpecGapField(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeDimension(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func normalizeResult(value string) string {
	value = normalizeDimension(value)
	switch value {
	case "not_applicable", "not applicable", "na":
		return coreharden.ResultNotApplicable
	default:
		return value
	}
}

func verifyCodeCitation(root string, label string, grounded string) []string {
	rel, lineNumber, ok := parseCodeGroundedIn(grounded)
	if !ok {
		return []string{fmt.Sprintf("%s: invalid code citation %q (expected code:<path>:<line>, for example code:src/file.go:42; line ranges are not supported)", label, grounded)}
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return []string{fmt.Sprintf("%s: cannot resolve workspace root: %v", label, err)}
	}
	candidate := filepath.Clean(filepath.Join(rootAbs, filepath.FromSlash(rel)))
	if !isInside(candidate, rootAbs) {
		return []string{fmt.Sprintf("%s: code citation escapes workspace root: %s", label, grounded)}
	}
	lines, err := countLines(candidate)
	if err != nil {
		return []string{fmt.Sprintf("%s: code citation not found: %s", label, grounded)}
	}
	if lineNumber > lines {
		return []string{fmt.Sprintf("%s: code citation line %d exceeds %d lines in %s", label, lineNumber, lines, rel)}
	}
	return nil
}

func verifyArchiveCitation(root string, label string, grounded string) []string {
	taskID := strings.TrimSpace(strings.TrimPrefix(grounded, "archive:"))
	if taskID == "" {
		return []string{fmt.Sprintf("%s: archive citation is empty (expected archive:<task-id>)", label)}
	}
	pattern := filepath.Join(root, ".scafld", "specs", "archive", "*", taskID+".md")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return []string{fmt.Sprintf("%s: archive citation not found: %s", label, grounded)}
	}
	return nil
}

func parseCodeGroundedIn(value string) (string, int, bool) {
	body := strings.TrimPrefix(value, "code:")
	rel, rawLine, ok := strings.Cut(body, ":")
	if !ok || rel == "" || rawLine == "" {
		return "", 0, false
	}
	lineNumber, err := strconv.Atoi(rawLine)
	if err != nil || lineNumber < 1 {
		return "", 0, false
	}
	return rel, lineNumber, true
}

func countLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

func isInside(path string, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func specPathIsDraft(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.Contains(normalized, "/.scafld/specs/drafts/") || strings.HasPrefix(normalized, ".scafld/specs/drafts/")
}

func fallback(value string, fb string) string {
	if strings.TrimSpace(value) == "" {
		return fb
	}
	return value
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }
