package harden

import (
	"encoding/json"
	"sort"
)

type schema map[string]any

// DossierSchemaJSON returns the semantic HardenDossier JSON Schema.
func DossierSchemaJSON() string {
	data, err := json.Marshal(dossierSchema(false))
	if err != nil {
		return "{}"
	}
	return string(data)
}

// StrictDossierSchemaJSON returns the Codex/OpenAI structured-output variant.
func StrictDossierSchemaJSON() string {
	data, err := json.Marshal(dossierSchema(true))
	if err != nil {
		return "{}"
	}
	return string(data)
}

func dossierSchema(strict bool) schema {
	root := strictObject(schema{
		"summary":      schema{"type": "string", "minLength": 1},
		"shape":        shapeSchema(strict),
		"observations": schema{"type": "array", "minItems": len(RequiredDimensions), "items": observationSchema(strict)},
	}, []string{"summary", "shape", "observations"}, strict)
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	root["title"] = "scafld HardenDossier provider response"
	return root
}

func shapeSchema(strict bool) schema {
	return strictObject(schema{
		"decision":            stringEnum(DecisionKeep, DecisionShrink, DecisionReframe, DecisionReject),
		"true_shape":          schema{"type": "string", "minLength": 1},
		"minimal_plan":        schema{"type": "string", "minLength": 1},
		"shared_owner":        schema{"type": "string", "minLength": 1},
		"adapter_boundaries":  schema{"type": "array", "items": schema{"type": "string", "minLength": 1}},
		"required_spec_edits": schema{"type": "array", "items": schema{"type": "string"}},
	}, []string{"decision", "true_shape", "minimal_plan", "shared_owner", "adapter_boundaries", "required_spec_edits"}, strict)
}

func observationSchema(strict bool) schema {
	return strictObject(schema{
		"dimension":     stringEnum(RequiredDimensions...),
		"result":        stringEnum(ResultClean, ResultAdvisory, ResultBlocks, ResultNotApplicable),
		"anchor":        schema{"type": "string", "minLength": 1},
		"note":          nullableString(),
		"question":      nullableString(),
		"recommended":   nullableString(),
		"if_unanswered": nullableString(),
		"default":       nullableString(),
		"status":        nullableStringEnum(StatusOpen, StatusFixed, StatusAcceptedRisk, StatusSuperseded),
	}, []string{"dimension", "result", "anchor"}, strict)
}

func strictObject(properties schema, required []string, allRequired bool) schema {
	if allRequired {
		required = sortedSchemaKeys(properties)
	} else if required == nil {
		required = []string{}
	}
	return schema{
		"type":                 "object",
		"additionalProperties": false,
		"required":             required,
		"properties":           properties,
	}
}

func nullableString() schema {
	return schema{"type": []string{"string", "null"}}
}

func stringEnum(values ...string) schema {
	enums := make([]any, 0, len(values))
	for _, value := range values {
		enums = append(enums, value)
	}
	return schema{"type": "string", "enum": enums}
}

func nullableStringEnum(values ...string) schema {
	enums := make([]any, 0, len(values)+1)
	for _, value := range values {
		enums = append(enums, value)
	}
	enums = append(enums, nil)
	return schema{"type": []string{"string", "null"}, "enum": enums}
}

func sortedSchemaKeys(properties schema) []string {
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
