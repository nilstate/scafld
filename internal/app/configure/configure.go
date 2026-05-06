package configure

import (
	"context"
	"sort"
)

// Scanner inspects a workspace without mutating it.
type Scanner interface {
	Scan(context.Context) (Snapshot, error)
}

// Snapshot is the evidence discovered from a workspace.
type Snapshot struct {
	Root       string
	Files      []Evidence
	Commands   []CommandSuggestion
	Invariants []InvariantSuggestion
	Warnings   []Warning
	Questions  []Question
}

// Evidence records one discovered project artifact.
type Evidence struct {
	Path string `json:"path" yaml:"path"`
	Role string `json:"role" yaml:"role"`
}

// CommandSuggestion is an inferred validation command with source evidence.
type CommandSuggestion struct {
	ID      string   `json:"id" yaml:"id"`
	Command string   `json:"command" yaml:"command"`
	Sources []string `json:"sources" yaml:"sources"`
}

// InvariantSuggestion is an inferred project invariant with source evidence.
type InvariantSuggestion struct {
	ID          string   `json:"id" yaml:"id"`
	Description string   `json:"description" yaml:"description"`
	Sources     []string `json:"sources" yaml:"sources"`
}

// Question records a missing policy decision that could not be inferred.
type Question struct {
	Question string   `json:"question" yaml:"question"`
	Reason   string   `json:"reason" yaml:"reason"`
	Sources  []string `json:"sources,omitempty" yaml:"sources,omitempty"`
}

// Warning records stale or suspicious project config state.
type Warning struct {
	ID      string   `json:"id" yaml:"id"`
	Message string   `json:"message" yaml:"message"`
	Sources []string `json:"sources,omitempty" yaml:"sources,omitempty"`
}

// Proposal is the written config proposal. It is intentionally not the runtime
// config schema; humans and agents must review it before applying changes.
type Proposal struct {
	Version       string       `json:"version" yaml:"version"`
	Purpose       string       `json:"purpose" yaml:"purpose"`
	Evidence      []Evidence   `json:"evidence" yaml:"evidence"`
	ConfigPatch   ConfigPatch  `json:"config_patch" yaml:"config_patch"`
	SpecGuidance  SpecGuidance `json:"spec_guidance,omitempty" yaml:"spec_guidance,omitempty"`
	Warnings      []Warning    `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	OpenQuestions []Question   `json:"open_questions,omitempty" yaml:"open_questions,omitempty"`
}

// ConfigPatch contains concrete changes that may be copied into config.yaml
// after review.
type ConfigPatch struct {
	Invariants map[string]string `json:"invariants" yaml:"invariants"`
}

// SpecGuidance contains repo facts useful to future specs, but not read from
// runtime config.
type SpecGuidance struct {
	Commands    []CommandSuggestion   `json:"commands,omitempty" yaml:"commands,omitempty"`
	ReviewFocus []InvariantSuggestion `json:"review_focus,omitempty" yaml:"review_focus,omitempty"`
}

// Output describes a configure run.
type Output struct {
	Path     string   `json:"path"`
	Proposal Proposal `json:"proposal"`
	Prompt   string   `json:"prompt"`
}

// Run builds a deterministic proposal from scanner evidence.
func Run(ctx context.Context, scanner Scanner) (Output, error) {
	snapshot, err := scanner.Scan(ctx)
	if err != nil {
		return Output{}, err
	}
	normalizeSnapshot(&snapshot)
	proposal := Proposal{
		Version:  "proposal-1",
		Purpose:  "Evidence-backed scafld configuration proposal. Review before copying anything into .scafld/config.yaml.",
		Evidence: snapshot.Files,
		ConfigPatch: ConfigPatch{
			Invariants: invariantMap(snapshot.Invariants),
		},
		SpecGuidance:  SpecGuidance{Commands: snapshot.Commands, ReviewFocus: snapshot.Invariants},
		Warnings:      snapshot.Warnings,
		OpenQuestions: snapshot.Questions,
	}
	return Output{Proposal: proposal, Prompt: prompt(proposal)}, nil
}

func normalizeSnapshot(snapshot *Snapshot) {
	sort.SliceStable(snapshot.Files, func(i, j int) bool {
		return snapshot.Files[i].Path < snapshot.Files[j].Path
	})
	sort.SliceStable(snapshot.Commands, func(i, j int) bool {
		return snapshot.Commands[i].ID < snapshot.Commands[j].ID
	})
	sort.SliceStable(snapshot.Invariants, func(i, j int) bool {
		return snapshot.Invariants[i].ID < snapshot.Invariants[j].ID
	})
	sort.SliceStable(snapshot.Warnings, func(i, j int) bool {
		return snapshot.Warnings[i].ID < snapshot.Warnings[j].ID
	})
	sort.SliceStable(snapshot.Questions, func(i, j int) bool {
		return snapshot.Questions[i].Question < snapshot.Questions[j].Question
	})
}

func invariantMap(invariants []InvariantSuggestion) map[string]string {
	if len(invariants) == 0 {
		return nil
	}
	out := make(map[string]string, len(invariants))
	for _, invariant := range invariants {
		out[invariant.ID] = invariant.Description
	}
	return out
}

func prompt(Proposal) string {
	return `# CONFIGURE MODE

scafld has written an evidence-backed proposal, not an applied config.

Your job:

1. Read .scafld/config.proposed.yaml.
2. Open every cited source before trusting a suggestion.
3. Copy only verified invariant IDs or local review defaults into .scafld/config.yaml.
4. Resolve open_questions by inspecting the repo or asking the operator.
5. Do not invent commands, policies, package managers, providers, or architecture rules.

The runtime config must stay truthful: if scafld does not read a field, do not
add it to .scafld/config.yaml as if it were enforced.
Treat commands and review focus as spec/review guidance unless you translate
them into real config fields that scafld already reads.
`
}
