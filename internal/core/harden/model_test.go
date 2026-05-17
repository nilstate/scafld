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

func TestParseTextRejectsPassWithQuestion(t *testing.T) {
	t.Parallel()

	text := validDossierJSON(VerdictPass)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatal(err)
	}
	payload["questions"] = []any{map[string]any{
		"question":           "Is this the right abstraction?",
		"grounded_in":        "spec_gap:Summary",
		"recommended_answer": "No until scope is narrowed.",
	}}
	data, _ := json.Marshal(payload)
	_, err := ParseText(string(data))
	if !errors.Is(err, ErrInvalidDossier) {
		t.Fatalf("err = %v", err)
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
	return `{"verdict":"` + verdict + `","summary":"clean","checks":[{"name":"path audit","grounded_in":"spec_gap:Scope","result":"passed","evidence":"checked"},{"name":"command audit","grounded_in":"spec_gap:Acceptance","result":"passed","evidence":"checked"},{"name":"scope/migration audit","grounded_in":"spec_gap:Scope","result":"passed","evidence":"checked"},{"name":"acceptance timing audit","grounded_in":"spec_gap:Phases","result":"passed","evidence":"checked"},{"name":"rollback/repair audit","grounded_in":"spec_gap:Rollback","result":"passed","evidence":"checked"},{"name":"design challenge","grounded_in":"spec_gap:Summary","result":"passed","evidence":"checked"}],"questions":[],"design_objections":[],"recommended_edits":[],"attack_log":[{"target":"draft","attack":"challenge","result":"clean"}]}`
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
