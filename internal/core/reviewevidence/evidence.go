// Package reviewevidence defines the canonical file evidence contract used by
// the receipt-grade review path.
package reviewevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"

	coreworkspace "github.com/nilstate/scafld/v2/internal/core/workspace"
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

// MaterialSeal records the reviewed task material independent of Git status.
// Scope is the normalized path set used to recompute Digest.
type MaterialSeal struct {
	Scope  []string
	Digest string
}

// MaterialFile records one present file's content hash under a material scope.
type MaterialFile struct {
	Path   string
	SHA256 string
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

// ComparisonSnapshot returns the Git-visible changes that review and complete
// use for stale-review checks. scafld runtime state is excluded because session
// and spec projection writes must not invalidate the review that produced them.
func ComparisonSnapshot(snapshot []string) []string {
	var kept []string
	for _, raw := range snapshot {
		if ComparisonPath(coreworkspace.ParseChange(raw).Path) {
			kept = append(kept, raw)
		}
	}
	return kept
}

// ComparisonPath reports whether path is part of the reviewed workspace state.
func ComparisonPath(raw string) bool {
	normalized := strings.Trim(strings.ReplaceAll(raw, "\\", "/"), "/")
	for _, prefix := range []string{
		".scafld/runs/",
		".scafld/specs/",
	} {
		if strings.HasPrefix(normalized+"/", prefix) {
			return false
		}
	}
	return true
}

// SnapshotDigest returns the stable digest sealed into review session entries.
func SnapshotDigest(snapshot []string) string {
	if len(snapshot) == 0 {
		return SHA256Hex([]byte("clean"))
	}
	return SHA256Hex([]byte(strings.Join(snapshot, "\n")))
}

// SnapshotDirty returns the string form stored in session JSON.
func SnapshotDirty(snapshot []string) string {
	if len(snapshot) == 0 {
		return "false"
	}
	return "true"
}

// MaterialDigest returns a content digest for the reviewed material. It ignores
// Git dirty status so dirty-to-committed transitions preserve authority when
// scoped file bytes are unchanged.
func MaterialDigest(scope []string, files []MaterialFile) string {
	normalizedScope := coreworkspace.NormalizeScope(scope)
	normalizedFiles := make([]MaterialFile, 0, len(files))
	for _, file := range files {
		path := strings.Trim(strings.ReplaceAll(strings.TrimSpace(file.Path), "\\", "/"), "/")
		path = strings.TrimPrefix(path, "./")
		sum := strings.ToLower(strings.TrimSpace(file.SHA256))
		if path == "" || sum == "" {
			continue
		}
		normalizedFiles = append(normalizedFiles, MaterialFile{Path: path, SHA256: sum})
	}
	sort.Slice(normalizedFiles, func(i, j int) bool {
		if normalizedFiles[i].Path == normalizedFiles[j].Path {
			return normalizedFiles[i].SHA256 < normalizedFiles[j].SHA256
		}
		return normalizedFiles[i].Path < normalizedFiles[j].Path
	})
	var b strings.Builder
	b.WriteString("scafld-reviewed-material-v1\n")
	b.WriteString("scope:\n")
	for _, item := range normalizedScope {
		b.WriteString(item)
		b.WriteByte('\n')
	}
	b.WriteString("files:\n")
	for _, file := range normalizedFiles {
		b.WriteString(file.Path)
		b.WriteByte('\t')
		b.WriteString(file.SHA256)
		b.WriteByte('\n')
	}
	return SHA256Hex([]byte(b.String()))
}
