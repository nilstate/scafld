package config

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
	Execution  *ExecutionSuggestion
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

// ExecutionSuggestion is an inferred acceptance-command environment.
type ExecutionSuggestion struct {
	PathPrepend []string          `json:"path_prepend,omitempty" yaml:"path_prepend,omitempty"`
	Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Sources     []string          `json:"sources" yaml:"sources"`
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
	Version           string            `json:"version" yaml:"version"`
	Purpose           string            `json:"purpose" yaml:"purpose"`
	AgentInstructions AgentInstructions `json:"agent_instructions" yaml:"agent_instructions"`
	Evidence          []Evidence        `json:"evidence" yaml:"evidence"`
	ConfigPatch       ConfigPatch       `json:"config_patch" yaml:"config_patch"`
	SpecGuidance      SpecGuidance      `json:"spec_guidance,omitempty" yaml:"spec_guidance,omitempty"`
	Warnings          []Warning         `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	OpenQuestions     []Question        `json:"open_questions,omitempty" yaml:"open_questions,omitempty"`
}

// AgentInstructions tells the consuming agent how to turn evidence into real
// project configuration without inventing unsupported runtime fields.
type AgentInstructions struct {
	Role         string   `json:"role" yaml:"role"`
	Deliverables []string `json:"deliverables" yaml:"deliverables"`
	Rules        []string `json:"rules" yaml:"rules"`
}

// ConfigPatch contains concrete changes that may be copied into config.yaml
// after review.
type ConfigPatch struct {
	Invariants map[string]string `json:"invariants" yaml:"invariants"`
	Execution  *ExecutionPatch   `json:"execution,omitempty" yaml:"execution,omitempty"`
}

// ExecutionPatch is the copyable runtime config fragment for acceptance commands.
type ExecutionPatch struct {
	PathPrepend []string          `json:"path_prepend,omitempty" yaml:"path_prepend,omitempty"`
	Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

// SpecGuidance contains repo facts useful to future specs, but not read from
// runtime config.
type SpecGuidance struct {
	Commands    []CommandSuggestion   `json:"commands,omitempty" yaml:"commands,omitempty"`
	ReviewFocus []InvariantSuggestion `json:"review_focus,omitempty" yaml:"review_focus,omitempty"`
	Execution   *ExecutionSuggestion  `json:"execution,omitempty" yaml:"execution,omitempty"`
}

// Output describes a config run.
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
		Version:           "proposal-1",
		Purpose:           "Evidence-backed instructions for configuring this repository for scafld.",
		AgentInstructions: agentInstructions(),
		Evidence:          snapshot.Files,
		ConfigPatch: ConfigPatch{
			Invariants: invariantMap(snapshot.Invariants),
			Execution:  executionPatch(snapshot.Execution),
		},
		SpecGuidance:  SpecGuidance{Commands: snapshot.Commands, ReviewFocus: snapshot.Invariants, Execution: snapshot.Execution},
		Warnings:      snapshot.Warnings,
		OpenQuestions: snapshot.Questions,
	}
	return Output{Proposal: proposal, Prompt: prompt(proposal)}, nil
}

func executionPatch(suggestion *ExecutionSuggestion) *ExecutionPatch {
	if suggestion == nil {
		return nil
	}
	return &ExecutionPatch{
		PathPrepend: append([]string(nil), suggestion.PathPrepend...),
		Env:         copyStringMap(suggestion.Env),
	}
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
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

func agentInstructions() AgentInstructions {
	return AgentInstructions{
		Role: "Configure this repository for scafld using only cited evidence.",
		Deliverables: []string{
			"Update .scafld/config.yaml with verified runtime config only: invariant IDs, execution environment, review provider defaults, timeouts, context, and review passes.",
			"Update AGENTS.md, CLAUDE.md, .claude/rules, or .scafld/prompts/* when the repo needs agent guidance that scafld should read as context but does not enforce as runtime config.",
			"Use spec_guidance.commands and spec_guidance.review_focus when drafting or tightening task specs; do not copy them into config unless they are translated into real config fields.",
			"Resolve open_questions by inspecting the repo or asking the operator before treating the answer as policy.",
		},
		Rules: []string{
			"Open every cited source before trusting a suggestion.",
			"Do not invent commands, package managers, providers, architecture rules, or policy names.",
			"Do not add fields to .scafld/config.yaml that the Go runtime does not read.",
			"Leave uncertain policy out of config and record the question instead of guessing.",
		},
	}
}

func prompt(Proposal) string {
	return `# CONFIG MODE

You are the configuration agent for this repository.

scafld has written .scafld/config.proposed.yaml as an instruction packet, not
an applied config. The real config still has to come from an agent or operator
that has inspected the repo.

1. Read .scafld/config.proposed.yaml.
2. Open every cited source before trusting a suggestion.
3. Update .scafld/config.yaml only with fields scafld actually enforces.
4. Update AGENTS.md, CLAUDE.md, .claude/rules, or .scafld/prompts/* when the
   finding belongs in agent guidance instead of runtime config.
5. Use spec_guidance for future specs and review passes; do not paste it into
   config as unsupported YAML.
6. Resolve open_questions by inspecting the repo or asking the operator.

The runtime config must stay truthful: if scafld does not read a field, do not
add it to .scafld/config.yaml as if it were enforced.
`
}
