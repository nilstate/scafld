package receipt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	// SchemaVersion is the current signed receipt schema.
	SchemaVersion = 1
	// SignatureAlgorithm is the detached-signature algorithm name.
	SignatureAlgorithm = "ed25519"
)

// Body is the canonical receipt payload signed by the host.
type Body struct {
	SchemaVersion             int               `json:"schema_version"`
	TaskID                    string            `json:"task_id"`
	SessionID                 string            `json:"session_id"`
	Verdict                   string            `json:"verdict"`
	BaseCommit                string            `json:"base_commit"`
	HeadCommit                string            `json:"head_commit"`
	Scope                     []string          `json:"scope"`
	TreeSHA                   string            `json:"tree_sha"`
	FileDigests               map[string]string `json:"file_digests"`
	IgnoredUnreviewed         []string          `json:"ignored_unreviewed"`
	ReviewedContextProvenance []Provenance      `json:"reviewed_context_provenance"`
	Reviewer                  Reviewer          `json:"reviewer"`
	HostUnderReview           HostUnderReview   `json:"host_under_review"`
	Independence              Independence      `json:"independence"`
	SpecFingerprint           string            `json:"spec_fingerprint"`
	Acceptance                []Acceptance      `json:"acceptance"`
	OpenBlockers              []Blocker         `json:"open_blockers"`
	MutationGuard             MutationGuard     `json:"mutation_guard"`
	LedgerHead                string            `json:"ledger_head"`
	MintedAt                  string            `json:"minted_at"`
}

// Provenance records one reviewed context source.
type Provenance struct {
	Kind   string `json:"kind"`
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
	Bytes  int    `json:"bytes,omitempty"`
}

// Reviewer records the independent reviewer runtime stamped into the receipt.
type Reviewer struct {
	Provider     string `json:"provider"`
	Model        string `json:"model,omitempty"`
	BinarySHA256 string `json:"binary_sha256,omitempty"`
	EndpointHost string `json:"endpoint_host,omitempty"`
}

// HostUnderReview identifies the host agent whose work is being gated.
type HostUnderReview struct {
	Agent     string `json:"agent"`
	Model     string `json:"model,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// Independence records the separation between the host and reviewer.
type Independence struct {
	Level    string `json:"level"`
	Distinct bool   `json:"distinct"`
}

// Acceptance records one acceptance command result included in the receipt.
type Acceptance struct {
	ID           string `json:"id"`
	PhaseID      string `json:"phase_id,omitempty"`
	Command      string `json:"command,omitempty"`
	ExpectedKind string `json:"expected_kind,omitempty"`
	Status       string `json:"status"`
	Reason       string `json:"reason,omitempty"`
	ExitCode     int    `json:"exit_code,omitempty"`
	OutputSHA256 string `json:"output_sha256,omitempty"`
	Diagnostic   string `json:"diagnostic,omitempty"`
}

// Blocker is a calibrated open reviewer finding.
type Blocker struct {
	ID         string `json:"id"`
	Severity   string `json:"severity,omitempty"`
	Location   string `json:"location,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Validation string `json:"validation,omitempty"`
}

// MutationGuard records the workspace mutation safety result.
type MutationGuard struct {
	Status string   `json:"status"`
	Scope  []string `json:"scope,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

// DetachedSignature is stored outside the canonical signed receipt body.
type DetachedSignature struct {
	Alg   string `json:"alg"`
	KeyID string `json:"key_id"`
	Sig   string `json:"sig"`
}

// Envelope carries a receipt body and its detached signature.
type Envelope struct {
	Body      Body              `json:"body"`
	Signature DetachedSignature `json:"signature"`
}

// CanonicalBody returns deterministic sorted-key JSON for a receipt body.
func CanonicalBody(body Body) ([]byte, error) {
	if err := ValidateBody(body); err != nil {
		return nil, err
	}
	return CanonicalJSON(body)
}

// CanonicalJSON returns compact deterministic JSON with lexicographically sorted object keys.
func CanonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal receipt value: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, fmt.Errorf("normalize receipt value: %w", err)
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, normalized); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// ReceiptDigest returns the hex sha256 digest used by the ledger chain.
// The digest excludes the detached signature and clears ledger_head to avoid
// making the head recursively depend on itself.
func ReceiptDigest(body Body) (string, error) {
	digestBody := body
	digestBody.LedgerHead = ""
	if err := validateBody(digestBody, false); err != nil {
		return "", err
	}
	canonical, err := CanonicalJSON(digestBody)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// ValidateBody checks required receipt body fields.
func ValidateBody(body Body) error {
	return validateBody(body, true)
}

func validateBody(body Body, requireLedgerHead bool) error {
	var missing []string
	if body.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version must be %d", SchemaVersion)
	}
	requiredString := []struct {
		name  string
		value string
	}{
		{"task_id", body.TaskID},
		{"session_id", body.SessionID},
		{"verdict", body.Verdict},
		{"base_commit", body.BaseCommit},
		{"head_commit", body.HeadCommit},
		{"tree_sha", body.TreeSHA},
		{"spec_fingerprint", body.SpecFingerprint},
		{"minted_at", body.MintedAt},
		{"reviewer.provider", body.Reviewer.Provider},
		{"host_under_review.agent", body.HostUnderReview.Agent},
		{"independence.level", body.Independence.Level},
		{"mutation_guard.status", body.MutationGuard.Status},
	}
	if requireLedgerHead {
		requiredString = append(requiredString, struct {
			name  string
			value string
		}{"ledger_head", body.LedgerHead})
	}
	for _, field := range requiredString {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	if body.Scope == nil {
		missing = append(missing, "scope")
	}
	if body.FileDigests == nil {
		missing = append(missing, "file_digests")
	}
	if body.IgnoredUnreviewed == nil {
		missing = append(missing, "ignored_unreviewed")
	}
	if body.ReviewedContextProvenance == nil {
		missing = append(missing, "reviewed_context_provenance")
	}
	if body.Acceptance == nil {
		missing = append(missing, "acceptance")
	}
	if body.OpenBlockers == nil {
		missing = append(missing, "open_blockers")
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing required receipt fields: %s", strings.Join(missing, ", "))
	}
	if err := ValidateReviewCoverage(body); err != nil {
		return err
	}
	switch body.Verdict {
	case "pass", "fail":
	default:
		return fmt.Errorf("invalid verdict %q", body.Verdict)
	}
	return nil
}

// ValidateReviewCoverage requires every signed file digest to be accounted for:
// either shown to the independent reviewer (present in reviewed_context_provenance)
// or explicitly declared withheld (ignored_unreviewed). A digest that is in
// neither would let a signed pass receipt imply the reviewer saw bytes it never
// did, breaking the accountability guarantee that the receipt covers the reviewed
// change set.
func ValidateReviewCoverage(body Body) error {
	covered := make(map[string]bool, len(body.ReviewedContextProvenance)+len(body.IgnoredUnreviewed))
	for _, p := range body.ReviewedContextProvenance {
		if p.Path != "" {
			covered[p.Path] = true
		}
	}
	for _, path := range body.IgnoredUnreviewed {
		covered[path] = true
	}
	var uncovered []string
	for path := range body.FileDigests {
		if !covered[path] {
			uncovered = append(uncovered, path)
		}
	}
	if len(uncovered) > 0 {
		sort.Strings(uncovered)
		return fmt.Errorf("file_digests not covered by review provenance or ignored_unreviewed: %s", strings.Join(uncovered, ", "))
	}
	return nil
}

// ValidateEnvelope checks the body and detached signature shape.
func ValidateEnvelope(envelope Envelope) error {
	if err := ValidateBody(envelope.Body); err != nil {
		return err
	}
	if envelope.Signature.Alg != SignatureAlgorithm {
		return fmt.Errorf("signature alg must be %q", SignatureAlgorithm)
	}
	if strings.TrimSpace(envelope.Signature.KeyID) == "" {
		return errors.New("signature key_id is required")
	}
	if strings.TrimSpace(envelope.Signature.Sig) == "" {
		return errors.New("signature sig is required")
	}
	return nil
}

// DecodeEnvelope parses a receipt envelope from JSON.
func DecodeEnvelope(data []byte) (Envelope, error) {
	var envelope Envelope
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return Envelope{}, fmt.Errorf("decode receipt envelope: %w", err)
	}
	if err := ValidateEnvelope(envelope); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func writeCanonical(out *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		if typed {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case string:
		data, _ := json.Marshal(typed)
		out.Write(data)
	case json.Number:
		out.WriteString(typed.String())
	case []any:
		out.WriteByte('[')
		for i, item := range typed {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			keyJSON, _ := json.Marshal(key)
			out.Write(keyJSON)
			out.WriteByte(':')
			if err := writeCanonical(out, typed[key]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical receipt value %T", value)
	}
	return nil
}
