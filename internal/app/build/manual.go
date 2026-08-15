package build

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	coreacceptance "github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

var (
	ErrCriterionNotFound       = errors.New("acceptance criterion not found")
	ErrManualCriterionRequired = errors.New("criterion is not manual")
	ErrEvidenceDisposition     = errors.New("evidence disposition must be pass or fail")
	ErrEvidenceDigest          = errors.New("evidence digest must be a SHA-256 hex digest")
	ErrEvidenceActor           = errors.New("evidence actor is required")
	ErrEvidenceReason          = errors.New("evidence reason is required")
)

// ManualEvidence is the operator-supplied disposition for one manual
// acceptance criterion. It is consumed as part of build, not as a separate
// lifecycle transition.
type ManualEvidence struct {
	CriterionID    string
	Disposition    string
	EvidenceDigest string
	Actor          string
	Reason         string
}

// NewManualEvidence returns nil when no manual disposition was supplied,
// preserving the normal build path.
func NewManualEvidence(criterionID, disposition, digest, actor, reason string) *ManualEvidence {
	if strings.TrimSpace(criterionID) == "" && strings.TrimSpace(disposition) == "" && strings.TrimSpace(digest) == "" && strings.TrimSpace(actor) == "" && strings.TrimSpace(reason) == "" {
		return nil
	}
	return &ManualEvidence{CriterionID: criterionID, Disposition: disposition, EvidenceDigest: digest, Actor: actor, Reason: reason}
}

func recordManualEvidence(ctx context.Context, sessions SessionStore, model spec.Model, ledger session.Session, input ManualEvidence, recordedAt string) (session.Session, bool, error) {
	criterion, ok := criterionByID(model, input.CriterionID)
	if !ok {
		return ledger, false, fmt.Errorf("%w: %s", ErrCriterionNotFound, strings.TrimSpace(input.CriterionID))
	}
	if criterion.Type != "manual" || criterion.ExpectedKind != coreacceptance.ExpectedManual {
		return ledger, false, fmt.Errorf("%w: %s", ErrManualCriterionRequired, criterion.ID)
	}
	disposition := strings.ToLower(strings.TrimSpace(input.Disposition))
	if disposition != "pass" && disposition != "fail" {
		return ledger, false, ErrEvidenceDisposition
	}
	digest, err := normalizeDigest(input.EvidenceDigest)
	if err != nil {
		return ledger, false, err
	}
	actor := strings.TrimSpace(input.Actor)
	if actor == "" {
		return ledger, false, ErrEvidenceActor
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return ledger, false, ErrEvidenceReason
	}
	if existing, ok := session.LatestManualEvidence(ledger, criterion.ID, criterion.PhaseID, string(criterion.ExpectedKind), criterion.Type); ok &&
		existing.Status == disposition && existing.EvidenceDigest == digest && existing.EvidenceActor == actor && existing.Reason == reason {
		return ledger, false, nil
	}
	updated, err := sessions.Append(ctx, model.TaskID, session.Entry{
		Type:           session.EntryManualEvidence,
		CriterionID:    criterion.ID,
		PhaseID:        criterion.PhaseID,
		Status:         disposition,
		Reason:         reason,
		ExpectedKind:   string(criterion.ExpectedKind),
		CriterionType:  criterion.Type,
		EvidenceDigest: digest,
		EvidenceActor:  actor,
	}, recordedAt)
	return updated, true, err
}

func criterionByID(model spec.Model, id string) (spec.Criterion, bool) {
	id = strings.TrimSpace(id)
	for _, criterion := range model.AllCriteria() {
		if criterion.ID == id {
			return criterion, true
		}
	}
	return spec.Criterion{}, false
}

func normalizeDigest(value string) (string, error) {
	value = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(value), "sha256:"))
	if len(value) != 64 {
		return "", ErrEvidenceDigest
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", ErrEvidenceDigest
	}
	return value, nil
}
