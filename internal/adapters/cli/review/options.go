package review

import (
	"strconv"
	"strings"

	appreview "github.com/nilstate/scafld/v2/internal/app/review"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
)

// InputOptions carries CLI review flags into the app input assembler.
type InputOptions struct {
	TaskID          string
	Mode            string
	ReviewScope     string
	MaxFindings     string
	MinAttackAngles string
	ReviewDepth     string
	ForceReview     bool
	PrintContext    bool
	HumanReviewed   bool
	Reason          string
}

// BuildInput maps CLI selection and flags to the app-layer review input.
func BuildInput(opts InputOptions, selected Selection) appreview.Input {
	return appreview.Input{
		TaskID:                  opts.TaskID,
		Mode:                    ModeFromValue(opts.Mode),
		ForceMode:               strings.TrimSpace(opts.Mode) != "",
		Passes:                  selected.Passes,
		Invariants:              selected.Invariants,
		ReviewScope:             SplitScope(opts.ReviewScope),
		ContextSections:         selected.ContextSections,
		ContextMaxBytes:         selected.ContextMaxBytes,
		RequiredContextMaxBytes: selected.RequiredContextMaxBytes,
		Contract:                selected.Contract,
		MaxFindings:             PositiveOrDefault(PositiveInt(opts.MaxFindings), selected.Dossier.MaxFindings),
		MinAttackAngles:         PositiveOrDefault(PositiveInt(opts.MinAttackAngles), selected.Dossier.MinAttackAngles),
		ReviewDepth:             FirstNonEmpty(opts.ReviewDepth, selected.Dossier.ReviewDepth),
		RerunPolicy:             selected.Dossier.RerunPolicy,
		ForceReview:             opts.ForceReview,
		ProviderName:            selected.ProviderName,
		ProviderModel:           selected.ProviderModel,
		PrintContext:            opts.PrintContext,
		HumanReviewed:           opts.HumanReviewed,
		Reason:                  opts.Reason,
	}
}

// ModeFromValue resolves the CLI mode value into the provider review mode.
func ModeFromValue(mode string) corereview.Mode {
	if strings.TrimSpace(mode) == string(corereview.ModeVerify) {
		return corereview.ModeVerify
	}
	return corereview.ModeDiscover
}

// PositiveInt parses non-negative integer CLI values. Invalid values use the
// unset value because review budgets are advisory, not gate inputs.
func PositiveInt(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// PositiveOrDefault returns fallback when value is unset.
func PositiveOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// FirstNonEmpty returns the first non-blank value.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
