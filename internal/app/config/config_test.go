package config

import (
	"context"
	"strings"
	"testing"
)

type fakeScanner struct {
	snapshot Snapshot
	err      error
}

func (f fakeScanner) Scan(context.Context) (Snapshot, error) {
	return f.snapshot, f.err
}

func TestRunBuildsEvidenceBackedProposal(t *testing.T) {
	t.Parallel()

	out, err := Run(context.Background(), fakeScanner{snapshot: Snapshot{
		Files: []Evidence{{Path: "Makefile", Role: "command_surface"}},
		Commands: []CommandSuggestion{{
			ID:      "full_check",
			Command: "make check",
			Sources: []string{"Makefile"},
		}},
		Invariants: []InvariantSuggestion{{
			ID:          "architecture_boundaries",
			Description: "Preserve architecture tests.",
			Sources:     []string{"internal/arch/architecture_test.go"},
		}},
		Execution: &ExecutionSuggestion{
			PathPrepend: []string{"$HOME/.rbenv/shims"},
			Sources:     []string{".ruby-version"},
		},
		Warnings: []Warning{{
			ID:      "ignored_config_keys",
			Message: "ignored config keys detected",
			Sources: []string{".scafld/config.yaml"},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Proposal.Version != "proposal-1" {
		t.Fatalf("proposal = %+v", out.Proposal)
	}
	if out.Proposal.AgentInstructions.Role == "" || len(out.Proposal.AgentInstructions.Deliverables) == 0 || len(out.Proposal.AgentInstructions.Rules) == 0 {
		t.Fatalf("agent instructions = %+v", out.Proposal.AgentInstructions)
	}
	if out.Proposal.ConfigPatch.Invariants["architecture_boundaries"] != "Preserve architecture tests." {
		t.Fatalf("invariants = %+v", out.Proposal.ConfigPatch.Invariants)
	}
	if len(out.Proposal.SpecGuidance.Commands) != 1 || out.Proposal.SpecGuidance.Commands[0].Command != "make check" {
		t.Fatalf("commands = %+v", out.Proposal.SpecGuidance.Commands)
	}
	if out.Proposal.ConfigPatch.Execution == nil || out.Proposal.ConfigPatch.Execution.PathPrepend[0] != "$HOME/.rbenv/shims" {
		t.Fatalf("execution suggestion = %+v", out.Proposal.ConfigPatch.Execution)
	}
	if out.Proposal.SpecGuidance.Execution == nil || out.Proposal.SpecGuidance.Execution.Sources[0] != ".ruby-version" {
		t.Fatalf("execution guidance = %+v", out.Proposal.SpecGuidance.Execution)
	}
	if out.Prompt == "" || !strings.Contains(out.Prompt, "configuration agent") || !strings.Contains(out.Prompt, ".claude/rules") {
		t.Fatalf("prompt missing agent instructions:\n%s", out.Prompt)
	}
	if len(out.Proposal.Warnings) != 1 || out.Proposal.Warnings[0].ID != "ignored_config_keys" {
		t.Fatalf("warnings = %+v", out.Proposal.Warnings)
	}
}
