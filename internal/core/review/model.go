package review

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type Severity string

const (
	SeverityBlocking    Severity = "blocking"
	SeverityNonBlocking Severity = "non_blocking"
)

const (
	VerdictPass = "pass"
	VerdictFail = "fail"
)

var ErrInvalidPacket = errors.New("invalid review packet")

type Finding struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Summary  string   `json:"summary"`
}

type Packet struct {
	Verdict      string         `json:"verdict"`
	Findings     []Finding      `json:"findings,omitempty"`
	Provider     string         `json:"provider,omitempty"`
	Model        string         `json:"model,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	EventSummary map[string]int `json:"event_summary,omitempty"`
	Raw          string         `json:"-"`
}

type Request struct {
	TaskID string
	Prompt string
}

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
			packet.Findings = append(packet.Findings, Finding{ID: "workspace_mutation", Severity: SeverityBlocking, Summary: "provider mutated workspace during review"})
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
	return packet, nil
}

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

func ValidatePacket(packet Packet) error {
	if !ValidVerdict(packet.Verdict) {
		return fmt.Errorf("%w: invalid verdict %q", ErrInvalidPacket, packet.Verdict)
	}
	for _, finding := range packet.Findings {
		if !ValidSeverity(finding.Severity) {
			return fmt.Errorf("%w: invalid severity %q", ErrInvalidPacket, finding.Severity)
		}
	}
	return nil
}

func VerdictFromFindings(findings []Finding) string {
	for _, finding := range findings {
		if finding.Severity == SeverityBlocking {
			return VerdictFail
		}
	}
	return VerdictPass
}

func ValidVerdict(verdict string) bool {
	switch verdict {
	case VerdictPass, VerdictFail:
		return true
	default:
		return false
	}
}

func ValidSeverity(severity Severity) bool {
	switch severity {
	case SeverityBlocking, SeverityNonBlocking:
		return true
	default:
		return false
	}
}
