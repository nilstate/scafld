package review

import (
	"errors"
	"testing"
)

func validDossier() Dossier {
	return Dossier{
		Verdict:   VerdictPass,
		Mode:      ModeDiscover,
		Summary:   "No open completion blockers.",
		Findings:  []Finding{},
		AttackLog: []AttackLogEntry{{Target: "diff", Attack: "regression scan", Result: "clean"}},
		Budget:    Budget{ActualAttackAngles: 1, Depth: "test"},
	}
}

func blockingFinding() Finding {
	return Finding{
		ID:               "bug",
		Severity:         SeverityHigh,
		BlocksCompletion: true,
		Category:         "regression",
		Confidence:       ConfidenceHigh,
		Location:         &Location{Path: "file.go", Line: 12},
		Evidence:         "file.go:12 drops the tenant id",
		Impact:           "cross-tenant cache invalidation can leak state",
		Validation:       "run go test ./internal/cache",
		Summary:          "tenant id omitted from cache key",
	}
}

func TestValidateDossierRequiresDossierShape(t *testing.T) {
	t.Parallel()

	if err := ValidateDossier(validDossier()); err != nil {
		t.Fatal(err)
	}
	for name, dossier := range map[string]Dossier{
		"invalid verdict":       {Verdict: "unknown", Mode: ModeDiscover, Summary: "x", AttackLog: []AttackLogEntry{{Target: "x", Attack: "x", Result: AttackResultClean}}},
		"invalid mode":          {Verdict: VerdictPass, Mode: "final", Summary: "x", AttackLog: []AttackLogEntry{{Target: "x", Attack: "x", Result: AttackResultClean}}},
		"missing summary":       {Verdict: VerdictPass, Mode: ModeDiscover, AttackLog: []AttackLogEntry{{Target: "x", Attack: "x", Result: AttackResultClean}}},
		"missing attacks":       {Verdict: VerdictPass, Mode: ModeDiscover, Summary: "x"},
		"invalid attack result": {Verdict: VerdictPass, Mode: ModeDiscover, Summary: "x", AttackLog: []AttackLogEntry{{Target: "x", Attack: "x", Result: "ok"}}},
	} {
		if err := ValidateDossier(dossier); !errors.Is(err, ErrInvalidDossier) {
			t.Fatalf("%s err = %v", name, err)
		}
	}
}

func TestSeverityAndCompletionBlockingAreSeparate(t *testing.T) {
	t.Parallel()

	dossier := validDossier()
	dossier.Findings = []Finding{{ID: "critical-note", Severity: SeverityCritical, BlocksCompletion: false, Summary: "critical risk accepted elsewhere"}}
	if err := ValidateDossier(dossier); err != nil {
		t.Fatal(err)
	}
	if dossier.Verdict != VerdictPass {
		t.Fatalf("verdict = %q", dossier.Verdict)
	}
}

func TestBlockingFindingsRequireRepairContext(t *testing.T) {
	t.Parallel()

	for field, mutate := range map[string]func(*Finding){
		"location":   func(f *Finding) { f.Location = nil },
		"evidence":   func(f *Finding) { f.Evidence = "" },
		"impact":     func(f *Finding) { f.Impact = "" },
		"validation": func(f *Finding) { f.Validation = "" },
	} {
		finding := blockingFinding()
		mutate(&finding)
		if err := ValidateFinding(finding); !errors.Is(err, ErrInvalidDossier) {
			t.Fatalf("%s err = %v", field, err)
		}
	}
}

func TestValidateDossierRejectsVerdictThatContradictsOpenBlockers(t *testing.T) {
	t.Parallel()

	dossier := validDossier()
	dossier.Verdict = VerdictPass
	dossier.Findings = []Finding{blockingFinding()}
	if err := ValidateDossier(dossier); !errors.Is(err, ErrInvalidDossier) {
		t.Fatalf("pass with blocking finding err = %v", err)
	}

	dossier = validDossier()
	dossier.Verdict = VerdictFail
	dossier.Findings = []Finding{{ID: "note", Severity: SeverityLow, BlocksCompletion: false, Summary: "note"}}
	if err := ValidateDossier(dossier); !errors.Is(err, ErrInvalidDossier) {
		t.Fatalf("fail without open blocker err = %v", err)
	}
}

func TestValidCompletionProviderAllowlist(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"codex", "claude", "command", "human", " codex "} {
		if !ValidCompletionProvider(provider) {
			t.Fatalf("provider %q should satisfy completion", provider)
		}
	}
	for _, provider := range []string{"", "local", "unknown"} {
		if ValidCompletionProvider(provider) {
			t.Fatalf("provider %q should not satisfy completion", provider)
		}
	}
}

func TestParseTextAcceptsDossierAndRejectsLegacyPacket(t *testing.T) {
	t.Parallel()

	dossier, err := ParseText(`{"verdict":"fail","mode":"discover","summary":"bug found","findings":[{"id":"bug","severity":"high","blocks_completion":true,"location":{"path":"file.go","line":12},"evidence":"file.go:12","impact":"breaks behavior","validation":"go test ./...","summary":"bug"}],"attack_log":[{"target":"diff","attack":"regression scan","result":"finding"}],"budget":{"actual_findings":1,"actual_attack_angles":1,"depth":"test"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if dossier.Verdict != VerdictFail || dossier.Findings[0].ID != "bug" {
		t.Fatalf("dossier = %+v", dossier)
	}
	_, err = ParseText(`{"verdict":"pass","findings":[]}`)
	if !errors.Is(err, ErrInvalidDossier) {
		t.Fatalf("legacy packet err = %v", err)
	}
	_, err = ParseText(`{"verdict":"pass","mode":"discover","summary":"clean","findings":[],"attack_log":[{"target":"diff","attack":"scan","result":"ok"}],"budget":{"actual_attack_angles":1}}`)
	if !errors.Is(err, ErrInvalidDossier) {
		t.Fatalf("invalid attack result err = %v", err)
	}
}

func TestParseNDJSONAcceptsDossierFrameAndRejectsLegacyFrames(t *testing.T) {
	t.Parallel()

	_, err := ParseNDJSON(`{"type":"dossier","dossier":{"verdict":"pass","mode":"discover","summary":"clean","findings":[],"attack_log":[{"target":"diff","attack":"scan","result":"clean"}],"budget":{"actual_attack_angles":1}}}` + "\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		`{"type":"verdict","verdict":"pass"}` + "\n",
		`{"type":"finding","severity":"high","summary":"bug"}` + "\n",
	} {
		if _, err := ParseNDJSON(input); !errors.Is(err, ErrInvalidDossier) {
			t.Fatalf("ParseNDJSON(%q) err = %v", input, err)
		}
	}
}
