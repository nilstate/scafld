package providers

import (
	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
	"github.com/nilstate/scafld/v2/internal/core/review"
)

// ReviewDossierSchemaJSON returns the strict structured-output schema used by
// providers that require every property to be listed in required.
func ReviewDossierSchemaJSON() string {
	return review.StrictDossierSchemaJSON()
}

// HardenDossierSchemaJSON returns the strict structured-output schema used by
// providers that require every property to be listed in required.
func HardenDossierSchemaJSON() string {
	return coreharden.StrictDossierSchemaJSON()
}
