package agentcontract

import "testing"

func TestHasProjectOverrideMarkerRequiresDirectiveLine(t *testing.T) {
	t.Parallel()

	if !HasProjectOverrideMarker([]byte("<!-- scafld:prompt-owner=project -->\n# Custom\n")) {
		t.Fatal("HTML comment directive marker was not recognized")
	}
	if !HasProjectOverrideMarker([]byte("# scafld:prompt-owner=project\n# Custom\n")) {
		t.Fatal("line comment directive marker was not recognized")
	}
	if HasProjectOverrideMarker([]byte("This file mentions `scafld:prompt-owner=project` in documentation.\n")) {
		t.Fatal("prose mention of marker activated project override")
	}
}
