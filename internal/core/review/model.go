package review

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Severity classifies whether a finding blocks completion.
type Severity string

const (
	// SeverityBlocking marks a finding that fails the review gate.
	SeverityBlocking Severity = "blocking"
	// SeverityNonBlocking marks a finding that does not fail the review gate.
	SeverityNonBlocking Severity = "non_blocking"
)

const (
	// VerdictPass marks a review packet with no blocking findings.
	VerdictPass = "pass"
	// VerdictFail marks a review packet with at least one blocking finding.
	VerdictFail = "fail"
)

// ErrInvalidPacket wraps malformed or semantically invalid provider output.
var ErrInvalidPacket = errors.New("invalid review packet")

// Finding is one issue surfaced by a review provider.
type Finding struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Summary  string   `json:"summary"`
}

// EncodeFindings serializes findings for session storage.
func EncodeFindings(findings []Finding) string {
	if len(findings) == 0 {
		return ""
	}
	data, err := json.Marshal(findings)
	if err != nil {
		return ""
	}
	return string(data)
}

// DecodeFindings parses findings recorded in a session review entry.
func DecodeFindings(text string) []Finding {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var findings []Finding
	if err := json.Unmarshal([]byte(text), &findings); err != nil {
		return nil
	}
	return findings
}

// CountBlocking returns the number of blocking findings.
func CountBlocking(findings []Finding) int {
	count := 0
	for _, finding := range findings {
		if finding.Severity == SeverityBlocking {
			count++
		}
	}
	return count
}

// ValidCompletionProvider reports whether a passing review provider can satisfy
// the completion gate for real work.
func ValidCompletionProvider(provider string) bool {
	switch strings.TrimSpace(provider) {
	case "codex", "claude", "command", "human":
		return true
	default:
		return false
	}
}

// Packet is the normalized review-provider payload consumed by scafld.
type Packet struct {
	Verdict      string         `json:"verdict"`
	Findings     []Finding      `json:"findings,omitempty"`
	Provider     string         `json:"provider,omitempty"`
	Model        string         `json:"model,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	EventSummary map[string]int `json:"event_summary,omitempty"`
	Raw          string         `json:"-"`
}

// Request is the provider-facing review prompt request.
type Request struct {
	TaskID string
	Prompt string
}

// ParseText parses direct JSON packets or NDJSON review streams.
func ParseText(text string) (Packet, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Packet{}, fmt.Errorf("%w: empty provider output", ErrInvalidPacket)
	}
	if strings.HasPrefix(trimmed, "{") {
		var probe map[string]json.RawMessage
		if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
			return Packet{}, fmt.Errorf("%w: %v", ErrInvalidPacket, err)
		}
		if _, hasType := probe["type"]; !hasType {
			var packet Packet
			if err := json.Unmarshal([]byte(trimmed), &packet); err != nil {
				return Packet{}, fmt.Errorf("%w: %v", ErrInvalidPacket, err)
			}
			packet.Raw = text
			packet = NormalizePacket(packet)
			if err := ValidatePacket(packet); err != nil {
				return Packet{}, err
			}
			return packet, nil
		}
	}
	return ParseNDJSON(text)
}

// ParseNDJSON parses newline-delimited review events into a packet.
func ParseNDJSON(text string) (Packet, error) {
	var packet Packet
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var frame struct {
			Type     string   `json:"type"`
			Verdict  string   `json:"verdict"`
			ID       string   `json:"id"`
			Severity Severity `json:"severity"`
			Summary  string   `json:"summary"`
		}
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			return Packet{}, fmt.Errorf("%w: %v", ErrInvalidPacket, err)
		}
		switch frame.Type {
		case "verdict", "done":
			if frame.Verdict != "" {
				if !ValidVerdict(frame.Verdict) {
					return Packet{}, fmt.Errorf("%w: invalid verdict %q", ErrInvalidPacket, frame.Verdict)
				}
				packet.Verdict = frame.Verdict
			}
		case "finding":
			if frame.ID == "" {
				frame.ID = fmt.Sprintf("finding-%d", len(packet.Findings)+1)
			}
			if frame.Severity == "" {
				frame.Severity = SeverityNonBlocking
			}
			if !ValidSeverity(frame.Severity) {
				return Packet{}, fmt.Errorf("%w: invalid severity %q", ErrInvalidPacket, frame.Severity)
			}
			packet.Findings = append(packet.Findings, Finding{ID: frame.ID, Severity: frame.Severity, Summary: frame.Summary})
		case "workspace_mutation":
			packet.Findings = append(packet.Findings, Finding{ID: "workspace_mutation", Severity: SeverityBlocking, Summary: "provider reported workspace mutation during review"})
		case "tick", "partial":
		default:
			return Packet{}, fmt.Errorf("%w: unknown frame type %q", ErrInvalidPacket, frame.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return Packet{}, fmt.Errorf("%w: %v", ErrInvalidPacket, err)
	}
	packet.Raw = text
	if packet.Verdict == "" {
		packet.Verdict = VerdictFromFindings(packet.Findings)
	}
	if err := ValidatePacket(packet); err != nil {
		return Packet{}, err
	}
	return packet, nil
}

// NormalizePacket fills derived packet defaults without changing findings.
func NormalizePacket(packet Packet) Packet {
	for i := range packet.Findings {
		if packet.Findings[i].ID == "" {
			packet.Findings[i].ID = fmt.Sprintf("finding-%d", i+1)
		}
		if packet.Findings[i].Severity == "" {
			packet.Findings[i].Severity = SeverityNonBlocking
		}
	}
	if packet.Verdict == "" {
		packet.Verdict = VerdictFromFindings(packet.Findings)
	}
	return packet
}

// ValidatePacket verifies verdict and finding severities.
func ValidatePacket(packet Packet) error {
	if !ValidVerdict(packet.Verdict) {
		return fmt.Errorf("%w: invalid verdict %q", ErrInvalidPacket, packet.Verdict)
	}
	for _, finding := range packet.Findings {
		if !ValidSeverity(finding.Severity) {
			return fmt.Errorf("%w: invalid severity %q", ErrInvalidPacket, finding.Severity)
		}
	}
	derived := VerdictFromFindings(packet.Findings)
	if packet.Verdict != derived {
		return fmt.Errorf("%w: verdict %q contradicts findings verdict %q", ErrInvalidPacket, packet.Verdict, derived)
	}
	return nil
}

// VerdictFromFindings derives the review verdict from finding severities.
func VerdictFromFindings(findings []Finding) string {
	for _, finding := range findings {
		if finding.Severity == SeverityBlocking {
			return VerdictFail
		}
	}
	return VerdictPass
}

// ValidVerdict reports whether verdict is supported by the review gate.
func ValidVerdict(verdict string) bool {
	switch verdict {
	case VerdictPass, VerdictFail:
		return true
	default:
		return false
	}
}

// ValidSeverity reports whether severity is supported by review packets.
func ValidSeverity(severity Severity) bool {
	switch severity {
	case SeverityBlocking, SeverityNonBlocking:
		return true
	default:
		return false
	}
}
