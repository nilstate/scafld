package harden

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseTextValidatesRequiredChecksAndDerivedVerdict(t *testing.T) {
	t.Parallel()

	dossier, err := ParseText(validDossierJSON(VerdictPass))
	if err != nil {
		t.Fatal(err)
	}
	if dossier.Verdict != VerdictPass || len(dossier.Checks) != len(RequiredCheckNames) {
		t.Fatalf("dossier = %+v", dossier)
	}
}

func TestParseTextRejectsPassWithOpenApprovalBlockingIssue(t *testing.T) {
	t.Parallel()

	text := validDossierJSON(VerdictPass)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatal(err)
	}
	payload["issues"] = []any{map[string]any{
		"id":              "harden-1",
		"kind":            "design_challenge",
		"severity":        "high",
		"blocks_approval": true,
		"status":          "open",
		"grounded_in":     "spec_gap:Summary",
		"summary":         "The plan may be future bloat.",
		"evidence":        "The spec does not cite repeated use.",
		"recommendation":  "Reduce scope or cite the repeated need.",
	}}
	data, _ := json.Marshal(payload)
	_, err := ParseText(string(data))
	if !errors.Is(err, ErrInvalidDossier) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseTextAllowsPassWithAdvisoryIssue(t *testing.T) {
	t.Parallel()

	text := validDossierJSON(VerdictPass)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatal(err)
	}
	payload["issues"] = []any{map[string]any{
		"id":                 "harden-1",
		"kind":               "question",
		"severity":           "low",
		"blocks_approval":    false,
		"status":             "open",
		"grounded_in":        "spec_gap:Rollback",
		"summary":            "Rollback could name a recovery command.",
		"evidence":           "Rollback exists but is terse.",
		"recommendation":     "Name the command if known.",
		"question":           "What should a human run if the cutover fails?",
		"recommended_answer": "Use the existing repair command.",
	}}
	data, _ := json.Marshal(payload)
	dossier, err := ParseText(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if dossier.Verdict != VerdictPass || len(dossier.Issues) != 1 {
		t.Fatalf("dossier = %+v", dossier)
	}
}

func TestHardenDossierSchemaIsStrictStructuredOutputCompatible(t *testing.T) {
	t.Parallel()

	var root map[string]any
	if err := json.Unmarshal([]byte(StrictDossierSchemaJSON()), &root); err != nil {
		t.Fatal(err)
	}
	assertStrictStructuredOutputSchema(t, "$", root)
}

func validDossierJSON(verdict string) string {
	return `{"verdict":"` + verdict + `","summary":"clean","checks":[{"name":"path audit","grounded_in":"spec_gap:Scope","result":"passed","evidence":"checked"},{"name":"command audit","grounded_in":"spec_gap:Acceptance","result":"passed","evidence":"checked"},{"name":"scope/migration audit","grounded_in":"spec_gap:Scope","result":"passed","evidence":"checked"},{"name":"acceptance timing audit","grounded_in":"spec_gap:Phases","result":"passed","evidence":"checked"},{"name":"rollback/repair audit","grounded_in":"spec_gap:Rollback","result":"passed","evidence":"checked"},{"name":"design challenge","grounded_in":"spec_gap:Summary","result":"passed","evidence":"checked"}],"issues":[],"attack_log":[{"target":"draft","attack":"challenge","result":"clean"}]}`
}

func assertStrictStructuredOutputSchema(t *testing.T, path string, node map[string]any) {
	t.Helper()
	if properties, ok := node["properties"].(map[string]any); ok {
		if node["additionalProperties"] != false {
			t.Fatalf("%s must set additionalProperties false", path)
		}
		required, ok := node["required"].([]any)
		if !ok {
			t.Fatalf("%s missing required", path)
		}
		if len(required) != len(properties) {
			t.Fatalf("%s must require every property", path)
		}
		for key, child := range properties {
			if nested, ok := child.(map[string]any); ok {
				assertStrictStructuredOutputSchema(t, path+"."+key, nested)
			}
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		assertStrictStructuredOutputSchema(t, path+"[]", items)
	}
}
