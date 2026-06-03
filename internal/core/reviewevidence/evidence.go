// Package reviewevidence defines the canonical file evidence contract used by
// the receipt-grade review path.
package reviewevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

const (
	StatusUnchanged = "unchanged"
	StatusAdded     = "added"
	StatusModified  = "modified"
	StatusDeleted   = "deleted"
	StatusGitlink   = "gitlink"
)

// EvidenceFile is one canonical repository file snapshot supplied to a
// receipt-grade reviewer sandbox.
type EvidenceFile struct {
	Path   string
	Status string
	Bytes  []byte
	SHA256 string
}

// Provenance records how one evidence file was materialized for review.
type Provenance struct {
	Path        string `json:"path"`
	Status      string `json:"status"`
	SHA256      string `json:"sha256"`
	ScratchPath string `json:"scratch_path"`
}

// NormalizePath returns a slash-separated repository-relative path.
func NormalizePath(raw string) (string, error) {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if value == "" {
		return "", fmt.Errorf("evidence path is required")
	}
	if strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("evidence path must be repository-relative: %s", raw)
	}
	clean := path.Clean(value)
	if clean == "." || clean == "" {
		return "", fmt.Errorf("evidence path is required")
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("evidence path escapes repository root: %s", raw)
	}
	return clean, nil
}

// ValidateStatus rejects evidence statuses outside the fingerprint enum.
func ValidateStatus(status string) error {
	switch strings.TrimSpace(status) {
	case StatusUnchanged, StatusAdded, StatusModified, StatusDeleted, StatusGitlink:
		return nil
	default:
		return fmt.Errorf("invalid evidence status %q", status)
	}
}

// ValidateFile normalizes path/status and verifies the declared byte hash.
func ValidateFile(file EvidenceFile) (EvidenceFile, error) {
	normalized, err := NormalizePath(file.Path)
	if err != nil {
		return EvidenceFile{}, err
	}
	status := strings.TrimSpace(file.Status)
	if err := ValidateStatus(status); err != nil {
		return EvidenceFile{}, err
	}
	file.Path = normalized
	file.Status = status
	file.SHA256 = strings.ToLower(strings.TrimSpace(file.SHA256))
	if err := VerifyHash(file); err != nil {
		return EvidenceFile{}, err
	}
	return file, nil
}

// VerifyHash checks that Bytes match the declared SHA-256 hex digest.
func VerifyHash(file EvidenceFile) error {
	want := strings.ToLower(strings.TrimSpace(file.SHA256))
	if want == "" {
		return fmt.Errorf("evidence sha256 is required for %s", file.Path)
	}
	got := SHA256Hex(file.Bytes)
	if got != want {
		return fmt.Errorf("evidence hash mismatch for %s: got %s want %s", file.Path, got, want)
	}
	return nil
}

// SHA256Hex returns the lowercase SHA-256 digest for data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
