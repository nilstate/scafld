// Package harden models provider-backed pre-approval hardening.
package harden

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
)

const (
	// VerdictPass means the draft contract is ready for approval.
	VerdictPass = "pass"
	// VerdictNeedsRevision means the draft needs contract edits before approval.
	VerdictNeedsRevision = "needs_revision"

	// ResultClean means the dimension was checked and no concern was found.
	ResultClean = "clean"
	// ResultAdvisory means the dimension has useful non-blocking feedback.
	ResultAdvisory = "advisory"
	// ResultBlocks means the dimension found work that must be resolved before approval.
	ResultBlocks = "blocks"
	// ResultNotApplicable means the dimension was checked and does not apply.
	ResultNotApplicable = "n/a"

	// StatusOpen marks an unresolved blocking observation.
	StatusOpen = "open"
	// StatusFixed marks a blocking observation resolved by a spec edit.
	StatusFixed = "fixed"
	// StatusAcceptedRisk marks a blocking observation intentionally accepted.
	StatusAcceptedRisk = "accepted_risk"
	// StatusSuperseded marks a blocking observation made irrelevant by later spec changes.
	StatusSuperseded = "superseded"
)

// ErrInvalidDossier wraps malformed or semantically invalid harden output.
var ErrInvalidDossier = errors.New("invalid harden dossier")

// RequiredDimensions are the hardening dimensions every round must cover.
var RequiredDimensions = []string{
	"design",
	"scope",
	"path",
	"command",
	"timing",
	"rollback",
}

// RequiredDimensionList formats the canonical dimension order for user-facing
// instructions and errors.
func RequiredDimensionList() string {
	return humanList(RequiredDimensions)
}

// Observation records one grounded hardening claim.
type Observation struct {
	Dimension string `json:"dimension"`
	Result    string `json:"result"`
	Anchor    string `json:"anchor"`
	Note      string `json:"note,omitempty"`
	Default   string `json:"default,omitempty"`
	Status    string `json:"status,omitempty"`
}

// Dossier is the normalized harden-provider payload consumed by scafld.
type Dossier struct {
	Summary      string         `json:"summary"`
	Observations []Observation  `json:"observations"`
	Provider     string         `json:"provider,omitempty"`
	Model        string         `json:"model,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	OutputFormat string         `json:"output_format,omitempty"`
	EventSummary map[string]int `json:"event_summary,omitempty"`
	Raw          string         `json:"-"`
}

// Request is the provider-facing hardening prompt request.
type Request struct {
	TaskID  string
	Prompt  string
	Context reviewcontext.Packet
}

// EncodeDossier serializes a dossier for transport and storage.
func EncodeDossier(dossier Dossier) string {
	if strings.TrimSpace(dossier.Summary) == "" && len(dossier.Observations) == 0 {
		return ""
	}
	data, err := json.Marshal(dossier)
	if err != nil {
		return ""
	}
	return string(data)
}

// ParseText parses one strict JSON HardenDossier.
func ParseText(text string) (Dossier, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Dossier{}, fmt.Errorf("%w: empty provider output", ErrInvalidDossier)
	}
	dossier, err := decodeDossierStrict([]byte(trimmed), text)
	if err != nil {
		return Dossier{}, err
	}
	if err := ValidateDossier(dossier); err != nil {
		return Dossier{}, err
	}
	return dossier, nil
}

func decodeDossierStrict(data []byte, raw string) (Dossier, error) {
	var dossier Dossier
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&dossier); err != nil {
		return Dossier{}, fmt.Errorf("%w: %v", ErrInvalidDossier, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Dossier{}, fmt.Errorf("%w: multiple JSON values", ErrInvalidDossier)
		}
		return Dossier{}, fmt.Errorf("%w: %v", ErrInvalidDossier, err)
	}
	dossier.Raw = raw
	return NormalizeDossier(dossier), nil
}

// NormalizeDossier fills derived defaults without hiding invalid provider shape.
func NormalizeDossier(dossier Dossier) Dossier {
	for i := range dossier.Observations {
		dossier.Observations[i].Dimension = normalize(dossier.Observations[i].Dimension)
		dossier.Observations[i].Result = normalizeResult(dossier.Observations[i].Result)
		if dossier.Observations[i].Result == ResultBlocks && strings.TrimSpace(dossier.Observations[i].Status) == "" {
			dossier.Observations[i].Status = StatusOpen
		}
		dossier.Observations[i].Status = normalize(dossier.Observations[i].Status)
	}
	return dossier
}

// ValidateDossier verifies provider output shape and the harden gate contract.
func ValidateDossier(dossier Dossier) error {
	if strings.TrimSpace(dossier.Summary) == "" {
		return fmt.Errorf("%w: summary is required", ErrInvalidDossier)
	}
	if len(dossier.Observations) == 0 {
		return fmt.Errorf("%w: observations must cover required harden dimensions", ErrInvalidDossier)
	}
	for _, observation := range dossier.Observations {
		dimension := normalize(observation.Dimension)
		if !ValidDimension(dimension) {
			return fmt.Errorf("%w: invalid observation dimension %q", ErrInvalidDossier, observation.Dimension)
		}
		if !ValidResult(observation.Result) {
			return fmt.Errorf("%w: invalid observation result %q", ErrInvalidDossier, observation.Result)
		}
		if strings.TrimSpace(observation.Anchor) == "" {
			return fmt.Errorf("%w: observations require anchor", ErrInvalidDossier)
		}
		if observationRequiresNote(observation) && strings.TrimSpace(observation.Note) == "" {
			return fmt.Errorf("%w: %s observations require note", ErrInvalidDossier, observation.Result)
		}
		if strings.TrimSpace(observation.Status) != "" && !ValidObservationStatus(observation.Status) {
			return fmt.Errorf("%w: invalid observation status %q", ErrInvalidDossier, observation.Status)
		}
	}
	if missing := MissingDimensions(dossier.Observations); len(missing) > 0 {
		return fmt.Errorf("%w: missing required harden dimensions: %s", ErrInvalidDossier, strings.Join(missing, ", "))
	}
	return nil
}

// VerdictFromDossier derives harden verdict from coverage and unresolved blocks.
func VerdictFromDossier(dossier Dossier) string {
	if len(MissingDimensions(dossier.Observations)) > 0 {
		return VerdictNeedsRevision
	}
	for _, observation := range dossier.Observations {
		if ObservationBlocksApproval(observation) {
			return VerdictNeedsRevision
		}
	}
	return VerdictPass
}

// MissingDimensions returns required dimensions not covered by the observations.
func MissingDimensions(observations []Observation) []string {
	seen := map[string]bool{}
	for _, observation := range observations {
		seen[normalize(observation.Dimension)] = true
	}
	var missing []string
	for _, required := range RequiredDimensions {
		if !seen[required] {
			missing = append(missing, required)
		}
	}
	return missing
}

// ObservationBlocksApproval reports whether an observation keeps harden not-ready.
func ObservationBlocksApproval(observation Observation) bool {
	if normalizeResult(observation.Result) != ResultBlocks {
		return false
	}
	switch normalize(observation.Status) {
	case "", StatusOpen:
		return true
	case StatusFixed, StatusAcceptedRisk, StatusSuperseded:
		return false
	default:
		return true
	}
}

// ValidVerdict reports whether the derived harden verdict is supported.
func ValidVerdict(verdict string) bool {
	switch strings.TrimSpace(verdict) {
	case VerdictPass, VerdictNeedsRevision:
		return true
	default:
		return false
	}
}

// ValidDimension reports whether a harden dimension is supported.
func ValidDimension(dimension string) bool {
	dimension = normalize(dimension)
	for _, required := range RequiredDimensions {
		if dimension == required {
			return true
		}
	}
	return false
}

// ValidResult reports whether an observation result is supported.
func ValidResult(result string) bool {
	switch normalizeResult(result) {
	case ResultClean, ResultAdvisory, ResultBlocks, ResultNotApplicable:
		return true
	default:
		return false
	}
}

// ValidObservationStatus reports whether an observation status is supported.
func ValidObservationStatus(status string) bool {
	switch normalize(status) {
	case StatusOpen, StatusFixed, StatusAcceptedRisk, StatusSuperseded:
		return true
	default:
		return false
	}
}

func observationRequiresNote(observation Observation) bool {
	switch normalizeResult(observation.Result) {
	case ResultAdvisory, ResultBlocks:
		return true
	default:
		return false
	}
}

func normalizeResult(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "not_applicable", "not applicable", "na":
		return ResultNotApplicable
	default:
		return strings.Join(strings.Fields(value), " ")
	}
}

func normalize(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func humanList(values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return values[0]
	case 2:
		return values[0] + " and " + values[1]
	default:
		return strings.Join(values[:len(values)-1], ", ") + ", and " + values[len(values)-1]
	}
}
