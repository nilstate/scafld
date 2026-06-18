package sessiontest

import (
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// PassingReviewDossier returns a minimal valid passing dossier for tests that
// need session-derived completion authority.
func PassingReviewDossier(provider string) corereview.Dossier {
	return corereview.Dossier{
		Verdict:  corereview.VerdictPass,
		Mode:     corereview.ModeVerify,
		Provider: provider,
		Summary:  "review passed",
		AttackLog: []corereview.AttackLogEntry{{
			Target: "diff",
			Attack: "scan",
			Result: corereview.AttackResultClean,
		}},
		Budget: corereview.Budget{ActualAttackAngles: 1},
	}
}

// PassingReviewEntry seals a passing non-human review the same way the runtime
// does, including review packet hash and reviewed workspace metadata.
func PassingReviewEntry(id string, provider string) session.Entry {
	dossier := PassingReviewDossier(provider)
	packet := corereview.EncodeDossier(dossier)
	return session.Entry{
		ID:                      id,
		Type:                    "review",
		Status:                  dossier.Verdict,
		Provider:                provider,
		Output:                  packet,
		ReviewPacket:            packet,
		CanonicalResponseSHA256: corereview.ResponseSHA256(packet),
		ProviderModel:           "test-model",
		ProviderSession:         "test-session",
		ReviewedHead:            "head",
		ReviewedDirty:           "true",
		ReviewedDiff:            corereview.ResponseSHA256("diff"),
		ReviewedSpec:            spec.ContractDigest(spec.Model{TaskID: "task"}),
	}
}
