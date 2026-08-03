// Package packetrepair writes operator-facing provider packet repair artifacts.
package packetrepair

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/providerpacket"
)

var (
	ErrIdentityMismatch      = errors.New("provider packet repair artifact identity mismatch")
	ErrMissingRepairedPacket = errors.New("provider packet repair artifact is missing repaired_packet")
)

type sourceCarrier interface {
	ProviderPacketRepairSource() providerpacket.Source
}

type Identity struct {
	TaskID, Gate, AttemptID, RoundID string
}

type ArtifactInput struct {
	Root, TaskID, Name, Gate, AttemptID, RoundID string
	Source                                       providerpacket.Source
	Err                                          error
	AcceptCommand                                func(path string) string
}

type Artifact struct {
	TaskID         string          `json:"task_id"`
	Gate           string          `json:"gate"`
	AttemptID      string          `json:"attempt_id,omitempty"`
	RoundID        string          `json:"round_id,omitempty"`
	Error          string          `json:"error,omitempty"`
	RejectedText   string          `json:"rejected_text,omitempty"`
	DiagnosticPath string          `json:"diagnostic_path,omitempty"`
	AcceptCommand  string          `json:"accept_command"`
	RepairedPacket json.RawMessage `json:"repaired_packet"`
}

func SourceFromError(err error) (providerpacket.Source, bool) {
	var carrier sourceCarrier
	if !errors.As(err, &carrier) {
		return providerpacket.Source{}, false
	}
	source := carrier.ProviderPacketRepairSource()
	return source, hasSource(source)
}

func Load(path string, identity Identity) ([]byte, error) {
	path = strings.TrimSpace(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	artifact, err := Decode(data)
	if err != nil {
		return nil, err
	}
	if !identity.matches(artifact) {
		return nil, fmt.Errorf("%w: expected task=%q gate=%q attempt=%q round=%q, got task=%q gate=%q attempt=%q round=%q", ErrIdentityMismatch, identity.TaskID, identity.Gate, identity.AttemptID, identity.RoundID, artifact.TaskID, artifact.Gate, artifact.AttemptID, artifact.RoundID)
	}
	packet := bytes.TrimSpace(artifact.RepairedPacket)
	if len(packet) == 0 || bytes.Equal(packet, []byte("null")) {
		return nil, ErrMissingRepairedPacket
	}
	return packet, nil
}

func (i Identity) matches(artifact Artifact) bool {
	if strings.TrimSpace(i.TaskID) != strings.TrimSpace(artifact.TaskID) || strings.TrimSpace(i.Gate) != strings.TrimSpace(artifact.Gate) {
		return false
	}
	if strings.TrimSpace(i.AttemptID) != "" && strings.TrimSpace(i.AttemptID) != strings.TrimSpace(artifact.AttemptID) {
		return false
	}
	if strings.TrimSpace(i.RoundID) != "" && strings.TrimSpace(i.RoundID) != strings.TrimSpace(artifact.RoundID) {
		return false
	}
	return true
}

func Decode(data []byte) (Artifact, error) {
	var artifact Artifact
	return artifact, json.Unmarshal(bytes.TrimSpace(data), &artifact)
}

func Encode(artifact Artifact) ([]byte, error) {
	if len(bytes.TrimSpace(artifact.RepairedPacket)) == 0 {
		artifact.RepairedPacket = json.RawMessage("null")
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func WriteArtifact(input ArtifactInput) (string, bool) {
	source := input.Source
	ok := hasSource(source)
	if extracted, hasExtracted := SourceFromError(input.Err); hasExtracted {
		source = extracted
		ok = true
	}
	if !ok {
		return "", false
	}
	if source.Error == "" && input.Err != nil {
		source.Error = input.Err.Error()
	}
	path := path(input.Root, input.TaskID, input.Name)
	command := ""
	if input.AcceptCommand != nil {
		command = input.AcceptCommand(path)
	}
	artifact := Artifact{
		TaskID:         strings.TrimSpace(input.TaskID),
		Gate:           strings.TrimSpace(input.Gate),
		AttemptID:      strings.TrimSpace(input.AttemptID),
		RoundID:        strings.TrimSpace(input.RoundID),
		Error:          strings.TrimSpace(source.Error),
		RejectedText:   strings.TrimSpace(source.RejectedText),
		DiagnosticPath: strings.TrimSpace(source.DiagnosticPath),
		AcceptCommand:  strings.TrimSpace(command),
		RepairedPacket: json.RawMessage("null"),
	}
	data, err := Encode(artifact)
	if err != nil {
		return "", false
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", false
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", false
	}
	return path, true
}

func path(root string, taskID string, name string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	dir := filepath.Join(root, ".scafld", "runs", strings.TrimSpace(taskID), "diagnostics")
	if strings.TrimSpace(name) == "" {
		name = "provider-packet-repair"
	}
	return filepath.Join(dir, safeName(name)+".json")
}

func RootFromSpecPath(path string) string {
	normalized := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if idx := strings.Index(normalized, "/.scafld/specs/"); idx >= 0 {
		return filepath.Clean(path[:idx])
	}
	if strings.HasPrefix(normalized, ".scafld/specs/") {
		return "."
	}
	return "."
}

func safeName(name string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, strings.TrimSpace(name))
	out := strings.Trim(mapped, "-.")
	if out == "" {
		return "provider-packet-repair"
	}
	return out
}

func hasSource(source providerpacket.Source) bool {
	return strings.TrimSpace(source.RejectedText) != ""
}
