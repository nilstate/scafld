package markdown

import (
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

func maximalModel() spec.Model {
	return spec.Model{
		Version:      "2.0",
		TaskID:       "probe-task",
		Created:      "2026-05-01T00:00:00Z",
		Updated:      "2026-05-02T00:00:00Z",
		Title:        "Probe task",
		Summary:      "Exercises every renderable field.",
		Status:       spec.StatusActive,
		HardenStatus: spec.HardenInProgress,
		Size:         spec.SizeLarge,
		RiskLevel:    spec.RiskHigh,
		CurrentState: spec.CurrentState{
			CurrentPhase:       "phase1",
			Next:               "review",
			Reason:             "runner update",
			Blockers:           "none",
			AllowedFollowUp:    "scafld review probe-task",
			LatestRunnerUpdate: "2026-05-02T00:00:00Z",
			ReviewGate:         "in_progress",
		},
		Context: spec.Context{
			CWD:           "/repo",
			Packages:      []string{"github.com/x/y"},
			FilesImpacted: []string{"src/a.ts", "src/b.ts"},
			Invariants:    []string{"no global state"},
			RelatedDocs:   []string{"docs/design.md"},
		},
		Objectives:   []string{"objective one", "objective two"},
		Scope:        []string{"parser", "renderer"},
		Dependencies: []string{"dep-a"},
		Assumptions:  []string{"assume-x"},
		Touchpoints:  []string{"CLI"},
		Risks: []spec.Risk{
			{Description: "data loss", Mitigation: "back up first"},
		},
		Acceptance: spec.Acceptance{
			ValidationProfile: "strict",
			Criteria: []spec.Criterion{
				{ID: "g1", Type: "command", Title: "global runs", Command: "true", ExpectedKind: acceptance.ExpectedExitCodeZero, Status: "pending"},
				{ID: "g2", Type: "command", Title: "global nonzero", Command: "false", ExpectedKind: acceptance.ExpectedExitCodeNonzero, Status: "pass", Evidence: "exit 1", SourceEvent: "e1"},
				{ID: "g3", Type: "manual", Title: "manual check", ExpectedKind: acceptance.ExpectedManual, Status: "pending"},
			},
		},
		Phases: []spec.Phase{
			{
				ID:             "phase1",
				Number:         1,
				Name:           "Implement",
				Status:         "active",
				Objective:      "do the thing",
				Changes:        []string{"added feature"},
				Dependencies:   []string{"phase0"},
				Acceptance: []spec.Criterion{
					{ID: "p1a", Type: "command", Title: "p1 command", Command: "go test ./...", ExpectedKind: acceptance.ExpectedExitCodeZero, Status: "pass", Evidence: "green", SourceEvent: "e2"},
					{ID: "p1b", Type: "browser", Title: "p1 browser", ExpectedKind: acceptance.ExpectedBrowserEvidence, Status: "pending"},
					{ID: "p1c", Type: "command", Title: "p1 nomatch", Command: "grep x", ExpectedKind: acceptance.ExpectedNoMatches, Status: "pending"},
				},
			},
			{
				ID:             "phase2",
				Number:         2,
				Name:           "Verify",
				Status:         "pending",
				Objective:      "verify the thing",
				Acceptance: []spec.Criterion{
					{ID: "p2a", Type: "command", Title: "p2 command", Command: "make verify", ExpectedKind: acceptance.ExpectedExitCodeZero, Status: "pending"},
				},
			},
		},
		Rollback:  []string{"revert commit"},
		SelfEval:  []string{"self eval note"},
		Deviations: []string{"none observed"},
		Review: spec.ReviewState{
			Status:       "completed",
			Verdict:      corereview.VerdictFail,
			Mode:         corereview.ModeDiscover,
			Summary:      "found blocker",
			Provider:     "claude",
			Model:        "claude-x",
			OutputFormat: "claude.mcp_submit_review",
			Normalizations: []string{"trim"},
			Findings: []corereview.Finding{
				{
					ID:               "f1",
					Severity:         corereview.SeverityHigh,
					BlocksCompletion: true,
					Status:           corereview.FindingOpen,
					Summary:          "bug here",
					Location:         &corereview.Location{Path: "src/a.ts", Line: 42},
					Evidence:         "ev",
					Impact:           "imp",
					Validation:       "val",
				},
				{
					ID:               "f2",
					Severity:         corereview.SeverityLow,
					BlocksCompletion: false,
					Summary:          "nit",
				},
			},
		},
		Metadata: map[string]string{"owner": "runtime", "ticket": "ABC-12"},
		Origin:   spec.Origin{CreatedBy: "tester", Source: "probe"},
		HardenRounds: []spec.HardenRound{
			{
				ID:        "round-1",
				Status:    "in_progress",
				StartedAt: "2026-05-01T00:00:00Z",
				Observations: []spec.HardenObservation{
					{Dimension: "path", Result: "clean", Anchor: "spec_gap:Scope", Note: "paths checked"},
					{Dimension: "rollback", Result: "advisory", Anchor: "spec_gap:Rollback", Default: "use repair", Status: "open"},
				},
			},
		},
	}
}

func TestProbeMaximalRoundTrip(t *testing.T) {
	model := maximalModel()
	rendered := string(Render(model))
	parsed, err := Parse([]byte(rendered))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	check := func(name, got, want string) {
		if got != want {
			t.Errorf("%s = %q, want %q\n--- rendered ---\n%s", name, got, want, rendered)
		}
	}
	check("Context.CWD", parsed.Context.CWD, "/repo")
	if len(parsed.Context.Packages) != 1 || parsed.Context.Packages[0] != "github.com/x/y" {
		t.Errorf("packages lost: %+v", parsed.Context.Packages)
	}
	if len(parsed.Context.FilesImpacted) != 2 || parsed.Context.FilesImpacted[0] != "src/a.ts" {
		t.Errorf("files impacted lost: %+v", parsed.Context.FilesImpacted)
	}
	if len(parsed.Context.Invariants) != 1 || parsed.Context.Invariants[0] != "no global state" {
		t.Errorf("invariants lost: %+v", parsed.Context.Invariants)
	}
	if len(parsed.Context.RelatedDocs) != 1 || parsed.Context.RelatedDocs[0] != "docs/design.md" {
		t.Errorf("related docs lost: %+v", parsed.Context.RelatedDocs)
	}
	if len(parsed.Risks) != 1 || parsed.Risks[0].Description != "data loss" || parsed.Risks[0].Mitigation != "back up first" {
		t.Errorf("risk lost: %+v", parsed.Risks)
	}
	if len(parsed.Acceptance.Criteria) != 3 {
		t.Errorf("global criteria count = %d, want 3: %+v", len(parsed.Acceptance.Criteria), parsed.Acceptance.Criteria)
	}
	byID := map[string]spec.Criterion{}
	for _, c := range parsed.Acceptance.Criteria {
		byID[c.ID] = c
	}
	if c, ok := byID["g2"]; !ok || c.ExpectedKind != acceptance.ExpectedExitCodeNonzero || c.Status != "pass" || c.Evidence != "exit 1" || c.SourceEvent != "e1" {
		t.Errorf("g2 lost: %+v", c)
	}
	if c, ok := byID["g3"]; !ok || c.Type != "manual" || c.ExpectedKind != acceptance.ExpectedManual {
		t.Errorf("g3 lost: %+v", c)
	}
	if len(parsed.Phases) != 2 {
		t.Fatalf("phase count = %d, want 2", len(parsed.Phases))
	}
	if parsed.Phases[1].ID != "phase2" || parsed.Phases[1].Number != 2 || parsed.Phases[1].Status != "pending" || parsed.Phases[1].Objective != "verify the thing" {
		t.Errorf("phase2 lost: %+v", parsed.Phases[1])
	}
	pc := map[string]spec.Criterion{}
	for _, c := range parsed.Phases[0].Acceptance {
		pc[c.ID] = c
	}
	if c, ok := pc["p1c"]; !ok || c.ExpectedKind != acceptance.ExpectedNoMatches {
		t.Errorf("p1c nomatch lost: %+v", c)
	}
	if c, ok := pc["p1b"]; !ok || c.Type != "browser" || c.ExpectedKind != acceptance.ExpectedBrowserEvidence {
		t.Errorf("p1b browser lost: %+v", c)
	}
	if parsed.Review.Normalizations == nil || parsed.Review.Normalizations[0] != "trim" {
		t.Errorf("normalizations lost: %+v", parsed.Review.Normalizations)
	}
	if len(parsed.Review.Findings) != 2 {
		t.Fatalf("findings count = %d, want 2", len(parsed.Review.Findings))
	}
	var f1 *corereview.Finding
	for i := range parsed.Review.Findings {
		if parsed.Review.Findings[i].ID == "f1" {
			f1 = &parsed.Review.Findings[i]
		}
	}
	if f1 == nil || f1.Location == nil || f1.Location.Line != 42 || f1.Evidence != "ev" || f1.Impact != "imp" || f1.Validation != "val" {
		t.Errorf("f1 detail lost: %+v", f1)
	}
	if parsed.Metadata["ticket"] != "ABC-12" {
		t.Errorf("metadata ticket lost: %+v", parsed.Metadata)
	}
	if parsed.HardenRounds[0].Observations[1].Default != "use repair" || parsed.HardenRounds[0].Observations[1].Status != "open" {
		t.Errorf("harden obs lost: %+v", parsed.HardenRounds[0].Observations)
	}
}
