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
