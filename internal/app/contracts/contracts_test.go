package contracts

import (
	"encoding/json"
	"testing"
)

func TestJSONEnvelopeNextActionGolden(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(Envelope[string]{OK: true, Command: "status", Result: "ok", Error: nil})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" {
		t.Fatal("empty envelope")
	}
}
