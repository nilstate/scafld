package harden

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestParseTextValidatesRequiredObservationsAndDerivedVerdict(t *testing.T) {
	t.Parallel()

	dossier, err := ParseText(validDossierJSON())
	if err != nil {
		t.Fatal(err)
	}
	if got := VerdictFromDossier(dossier); got != VerdictPass || len(dossier.Observations) != len(RequiredDimensions) {
		t.Fatalf("verdict=%s dossier=%+v", got, dossier)
	}
}

func TestParseTextRejectsOldSelfReportedVerdictShape(t *testing.T) {
	t.Parallel()

	_, err := ParseText(`{"verdict":"pass","summary":"clean","observations":[]}`)
	if !errors.Is(err, ErrInvalidDossier) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseTextDerivesNeedsRevisionFromOpenBlock(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	if err := json.Unmarshal([]byte(validDossierJSON()), &payload); err != nil {
		t.Fatal(err)
	}
	observations := payload["observations"].([]any)
	observations[0] = map[string]any{
		"dimension": "design",
		"result":    "blocks",
		"anchor":    "spec_gap:Summary",
		"note":      "The plan may be future bloat.",
		"default":   "Reduce scope or cite the repeated need.",
		"status":    "open",
	}
	data, _ := json.Marshal(payload)
	dossier, err := ParseText(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := VerdictFromDossier(dossier); got != VerdictNeedsRevision {
		t.Fatalf("verdict = %s", got)
	}
}

func TestParseTextAllowsAdvisoryObservation(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	if err := json.Unmarshal([]byte(validDossierJSON()), &payload); err != nil {
		t.Fatal(err)
	}
	observations := payload["observations"].([]any)
	observations[5] = map[string]any{
		"dimension": "rollback",
		"result":    "advisory",
		"anchor":    "spec_gap:Rollback",
		"note":      "Rollback could name a recovery command.",
		"default":   "Use the existing repair command.",
		"status":    "",
	}
	data, _ := json.Marshal(payload)
	dossier, err := ParseText(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := VerdictFromDossier(dossier); got != VerdictPass {
		t.Fatalf("verdict = %s", got)
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

func TestRequiredDimensionsPrioritizeArchitecture(t *testing.T) {
	t.Parallel()

	want := []string{"design", "scope", "path", "command", "timing", "rollback"}
	if !reflect.DeepEqual(RequiredDimensions, want) {
		t.Fatalf("RequiredDimensions = %#v, want %#v", RequiredDimensions, want)
	}
	if got := RequiredDimensionList(); got != "design, scope, path, command, timing, and rollback" {
		t.Fatalf("RequiredDimensionList() = %q", got)
	}
}

func validDossierJSON() string {
	return `{"summary":"clean","observations":[{"dimension":"design","result":"clean","anchor":"spec_gap:Summary"},{"dimension":"scope","result":"clean","anchor":"spec_gap:Scope"},{"dimension":"path","result":"clean","anchor":"spec_gap:Scope"},{"dimension":"command","result":"clean","anchor":"spec_gap:Acceptance"},{"dimension":"timing","result":"clean","anchor":"spec_gap:Phases"},{"dimension":"rollback","result":"n/a","anchor":"spec_gap:Rollback"}]}`
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
