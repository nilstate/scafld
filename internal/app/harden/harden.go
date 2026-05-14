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

	"github.com/nilstate/scafld/v2/internal/core/gate"
	coreprompts "github.com/nilstate/scafld/v2/internal/core/prompts"
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
)

const groundedInShape = `expected "Grounded in: spec_gap:<field>", "Grounded in: code:<path>:<line>", or "Grounded in: archive:<task-id>"`

var requiredHardenChecks = []string{
	"path audit",
	"command audit",
	"scope/migration audit",
	"acceptance timing audit",
	"rollback/repair audit",
	"design challenge",
}

// SpecStore is the spec persistence port used by hardening.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

// Clock supplies hardening timestamps.
type Clock interface {
	Now() time.Time
}

// Input describes a hardening operation.
type Input struct {
	TaskID     string
	MarkPassed bool
	Root       string
	Prompt     string
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
	model, path, err := store.Load(ctx, input.TaskID)
	if err != nil {
		return Output{}, err
	}
	if !specPathIsDraft(path) {
		return Output{}, fmt.Errorf("%w: %s", ErrSpecNotDraft, path)
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	if input.MarkPassed {
		return markPassed(ctx, store, path, model, now, input.Root)
	}
	return openRound(ctx, store, path, model, now, fallback(input.Prompt, coreprompts.Harden))
}

func openRound(ctx context.Context, store SpecStore, path string, model spec.Model, now string, prompt string) (Output, error) {
	roundID := nextRoundID(model.HardenRounds)
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = append(model.HardenRounds, spec.HardenRound{
		ID:        roundID,
		Status:    string(spec.HardenInProgress),
		StartedAt: now,
	})
	model.Updated = now
	model.CurrentState.Next = "harden"
	model.CurrentState.Reason = "hardening round in progress"
	model.CurrentState.Blockers = "none"
	model.CurrentState.AllowedFollowUp = "scafld harden " + model.TaskID + " --mark-passed"
	if err := store.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save harden round: %w", err)
	}
	return Output{
		TaskID:       model.TaskID,
		Path:         path,
		HardenStatus: model.HardenStatus,
		RoundID:      roundID,
		NextCommand:  model.CurrentState.AllowedFollowUp,
		Prompt:       prompt,
	}, nil
}

func markPassed(ctx context.Context, store SpecStore, path string, model spec.Model, now string, root string) (Output, error) {
	if len(model.HardenRounds) == 0 {
		return Output{}, ErrNoHardenRound
	}
	latest := len(model.HardenRounds) - 1
	warnings := verifyHardenRoundEvidence(root, model.HardenRounds[latest])
	if len(warnings) > 0 {
		out := Output{TaskID: model.TaskID, Path: path, HardenStatus: model.HardenStatus, RoundID: model.HardenRounds[latest].ID, Warnings: warnings}
		return out, gate.New(ErrInvalidHardenEvidence, gate.Failure{
			Gate:     "harden",
			Status:   string(model.Status),
			Reason:   "hardening evidence is incomplete",
			Evidence: warnings,
			Expected: "required harden checks with grounded evidence, plus resolved questions when questions exist",
			Actual:   strings.Join(warnings, "; "),
			Blockers: warnings,
			Next:     "fix the harden checks/questions, then run scafld harden " + model.TaskID + " --mark-passed",
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

func verifyHardenRoundEvidence(root string, round spec.HardenRound) []string {
	if root == "" {
		root = "."
	}
	var warnings []string
	warnings = append(warnings, verifyHardenChecks(root, round.Checks)...)
	for i, question := range round.Questions {
		idx := i + 1
		warnings = append(warnings, verifyGroundedIn(root, fmt.Sprintf("question %d", idx), groundedInShape, question.GroundedIn)...)
		if strings.TrimSpace(question.RecommendedAnswer) == "" {
			warnings = append(warnings, fmt.Sprintf("question %d: missing recommended answer", idx))
		}
		if strings.TrimSpace(question.AnsweredWith) == "" {
			warnings = append(warnings, fmt.Sprintf("question %d: missing answered with resolution", idx))
		}
	}
	return warnings
}

func verifyHardenChecks(root string, checks []spec.HardenCheck) []string {
	var warnings []string
	if len(checks) == 0 {
		return []string{"missing harden checks: record path audit, command audit, scope/migration audit, acceptance timing audit, rollback/repair audit, and design challenge before marking hardening passed"}
	}
	seen := map[string]bool{}
	for i, check := range checks {
		idx := i + 1
		name := strings.TrimSpace(check.Name)
		if name == "" {
			warnings = append(warnings, fmt.Sprintf("check %d: missing name", idx))
			continue
		}
		normalized := normalizeCheckName(name)
		seen[normalized] = true
		label := fmt.Sprintf("check %q", name)
		warnings = append(warnings, verifyGroundedIn(root, label, groundedInShape, check.GroundedIn)...)
		if strings.TrimSpace(check.Evidence) == "" {
			warnings = append(warnings, fmt.Sprintf("%s: missing evidence", label))
		}
		switch result := strings.TrimSpace(strings.ToLower(check.Result)); result {
		case "passed", "not_applicable":
		default:
			warnings = append(warnings, fmt.Sprintf("%s: result must be passed or not_applicable before marking hardening passed", label))
		}
	}
	var missing []string
	for _, required := range requiredHardenChecks {
		if !seen[required] {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		warnings = append(warnings, fmt.Sprintf("missing required harden checks: %s (record each under Checks with Grounded in, Result, and Evidence)", strings.Join(missing, ", ")))
	}
	return warnings
}

func verifyGroundedIn(root string, label string, expected string, grounded string) []string {
	grounded = strings.TrimSpace(grounded)
	switch {
	case grounded == "":
		return []string{fmt.Sprintf("%s: missing grounded_in (%s)", label, expected)}
	case strings.HasPrefix(grounded, "spec_gap:"):
		return nil
	case strings.HasPrefix(grounded, "code:"):
		return verifyCodeCitation(root, label, grounded)
	case strings.HasPrefix(grounded, "archive:"):
		return verifyArchiveCitation(root, label, grounded)
	default:
		return []string{fmt.Sprintf("%s: invalid grounded_in prefix %q (%s)", label, grounded, expected)}
	}
}

func normalizeCheckName(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
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
