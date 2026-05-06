package configure

import (
	"context"
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
		Warnings: []Warning{{
			ID:      "legacy_ignored_config_keys",
			Message: "legacy config keys detected",
			Sources: []string{".scafld/config.yaml"},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Proposal.Version != "proposal-1" {
		t.Fatalf("proposal = %+v", out.Proposal)
	}
	if out.Proposal.ConfigPatch.Invariants["architecture_boundaries"] != "Preserve architecture tests." {
		t.Fatalf("invariants = %+v", out.Proposal.ConfigPatch.Invariants)
	}
	if len(out.Proposal.SpecGuidance.Commands) != 1 || out.Proposal.SpecGuidance.Commands[0].Command != "make check" {
		t.Fatalf("commands = %+v", out.Proposal.SpecGuidance.Commands)
	}
	if out.Prompt == "" {
		t.Fatal("prompt was empty")
	}
	if len(out.Proposal.Warnings) != 1 || out.Proposal.Warnings[0].ID != "legacy_ignored_config_keys" {
		t.Fatalf("warnings = %+v", out.Proposal.Warnings)
	}
}
