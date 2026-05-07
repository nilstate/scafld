package review

import (
	"errors"
	"testing"
)

func TestParseNDJSONRejectsInvalidVerdictAndSeverity(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		`{"type":"verdict","verdict":"maybe"}` + "\n",
		`{"type":"finding","severity":"major","summary":"bug"}` + "\n",
	} {
		_, err := ParseNDJSON(input)
		if !errors.Is(err, ErrInvalidPacket) {
			t.Fatalf("ParseNDJSON(%q) err = %v", input, err)
		}
	}
}

func TestValidatePacketClassifiesDirectProviderOutput(t *testing.T) {
	t.Parallel()

	if err := ValidatePacket(Packet{Verdict: VerdictPass}); err != nil {
		t.Fatal(err)
	}
	err := ValidatePacket(Packet{Verdict: "unknown"})
	if !errors.Is(err, ErrInvalidPacket) {
		t.Fatalf("invalid verdict err = %v", err)
	}
}

func TestValidatePacketRejectsVerdictThatContradictsFindings(t *testing.T) {
	t.Parallel()

	err := ValidatePacket(Packet{
		Verdict: VerdictPass,
		Findings: []Finding{{
			ID:       "bug",
			Severity: SeverityBlocking,
			Summary:  "blocking defect",
		}},
	})
	if !errors.Is(err, ErrInvalidPacket) {
		t.Fatalf("pass with blocking finding err = %v", err)
	}

	err = ValidatePacket(Packet{
		Verdict:  VerdictFail,
		Findings: []Finding{{ID: "note", Severity: SeverityNonBlocking, Summary: "note"}},
	})
	if !errors.Is(err, ErrInvalidPacket) {
		t.Fatalf("fail without blocking finding err = %v", err)
	}
}

func TestValidCompletionProviderAllowlist(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"codex", "claude", "command", " codex "} {
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

func TestParseNDJSONRejectsContradictoryFinalVerdict(t *testing.T) {
	t.Parallel()

	input := `{"type":"finding","id":"bug","severity":"blocking","summary":"bug"}` + "\n" +
		`{"type":"verdict","verdict":"pass"}` + "\n"
	_, err := ParseNDJSON(input)
	if !errors.Is(err, ErrInvalidPacket) {
		t.Fatalf("contradictory ndjson err = %v", err)
	}
}

func TestParseTextAcceptsPacketJSONAndRejectsEmptyOutput(t *testing.T) {
	t.Parallel()

	packet, err := ParseText(`{"findings":[{"severity":"blocking","summary":"bug"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Verdict != VerdictFail || packet.Findings[0].ID != "finding-1" {
		t.Fatalf("packet = %+v", packet)
	}
	_, err = ParseText("")
	if !errors.Is(err, ErrInvalidPacket) {
		t.Fatalf("empty output err = %v", err)
	}
}
