package providers

import "github.com/nilstate/scafld/v2/internal/core/review"

// ReviewDossierSchemaJSON returns the strict structured-output schema used by
// providers that require every property to be listed in required.
func ReviewDossierSchemaJSON() string {
	return review.StrictDossierSchemaJSON()
}
