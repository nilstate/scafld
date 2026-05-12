package providers

import (
	"encoding/json"
	"sort"
)

type schema map[string]any

// ReviewDossierSchemaJSON returns the provider-facing review dossier schema.
//
// Codex/OpenAI structured outputs require every object property to be listed in
// required, even when the property is semantically optional. Optional fields are
// therefore represented as nullable required fields. Provenance fields such as
// provider, model, session_id, output_format, and event_summary are filled by
// scafld after the provider returns and are intentionally not part of this
// schema.
func ReviewDossierSchemaJSON() string {
	data, err := json.Marshal(reviewDossierSchema())
	if err != nil {
		return "{}"
	}
	return string(data)
}

func reviewDossierSchema() schema {
	root := strictObject(schema{
		"verdict":    stringEnum("pass", "fail"),
		"mode":       stringEnum("discover", "verify"),
		"summary":    schema{"type": "string", "minLength": 1},
		"findings":   schema{"type": "array", "items": findingSchema()},
		"attack_log": schema{"type": "array", "minItems": 1, "items": attackLogSchema()},
		"budget":     budgetSchema(),
	})
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	root["title"] = "scafld ReviewDossier provider response"
	return root
}

func findingSchema() schema {
	return strictObject(schema{
		"id":                schema{"type": "string"},
		"severity":          stringEnum("critical", "high", "medium", "low"),
		"blocks_completion": schema{"type": "boolean"},
		"category":          nullableString(),
		"confidence":        nullableStringEnum("high", "medium", "low"),
		"location": nullableStrictObject(schema{
			"path": schema{"type": "string"},
			"line": nullableInteger(schema{"minimum": 1}),
		}),
		"evidence":      nullableString(),
		"impact":        nullableString(),
		"reproducer":    nullableString(),
		"suggested_fix": nullableString(),
		"validation":    nullableString(),
		"related_spec":  nullableString(),
		"review_pass":   nullableString(),
		"status":        nullableStringEnum("open", "fixed", "accepted_risk", "superseded"),
		"summary":       nullableString(),
	})
}

func attackLogSchema() schema {
	return strictObject(schema{
		"target": schema{"type": "string"},
		"attack": schema{"type": "string"},
		"result": stringEnum("finding", "clean", "skipped"),
		"notes":  nullableString(),
	})
}

func budgetSchema() schema {
	return strictObject(schema{
		"max_findings":         nullableInteger(nil),
		"min_attack_angles":    nullableInteger(nil),
		"actual_findings":      nullableInteger(nil),
		"actual_attack_angles": nullableInteger(nil),
		"depth":                nullableString(),
	})
}

func strictObject(properties schema) schema {
	return schema{
		"type":                 "object",
		"additionalProperties": false,
		"required":             sortedSchemaKeys(properties),
		"properties":           properties,
	}
}

func nullableStrictObject(properties schema) schema {
	out := strictObject(properties)
	out["type"] = []string{"object", "null"}
	return out
}

func nullableString() schema {
	return schema{"type": []string{"string", "null"}}
}

func nullableInteger(extra schema) schema {
	out := schema{"type": []string{"integer", "null"}}
	for key, value := range extra {
		out[key] = value
	}
	return out
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
