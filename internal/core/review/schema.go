package review

import (
	"encoding/json"
	"sort"
)

type schema map[string]any

// DossierSchemaJSON returns the semantic ReviewDossier JSON Schema.
//
// This schema is used for MCP tool input, managed docs, and operator-facing
// validation. It marks only semantically required fields as required while
// keeping additionalProperties false throughout the dossier tree.
func DossierSchemaJSON() string {
	data, err := json.Marshal(dossierSchema(false))
	if err != nil {
		return "{}"
	}
	return string(data)
}

// StrictDossierSchemaJSON returns the Codex/OpenAI structured-output variant.
//
// OpenAI structured outputs require every object property to be listed in
// required, even when the property is semantically optional. Optional fields are
// therefore represented as nullable required fields.
func StrictDossierSchemaJSON() string {
	data, err := json.Marshal(dossierSchema(true))
	if err != nil {
		return "{}"
	}
	return string(data)
}

func dossierSchema(strict bool) schema {
	root := strictObject(schema{
		"verdict":    stringEnum("pass", "fail"),
		"mode":       stringEnum("discover", "verify"),
		"summary":    schema{"type": "string", "minLength": 1},
		"findings":   schema{"type": "array", "items": findingSchema(strict)},
		"attack_log": schema{"type": "array", "minItems": 1, "items": attackLogSchema(strict)},
		"budget":     budgetSchema(strict),
	}, []string{"verdict", "mode", "summary", "findings", "attack_log", "budget"}, strict)
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	root["title"] = "scafld ReviewDossier provider response"
	return root
}

func findingSchema(strict bool) schema {
	return strictObject(schema{
		"id":                schema{"type": "string"},
		"severity":          stringEnum("critical", "high", "medium", "low"),
		"blocks_completion": schema{"type": "boolean"},
		"category":          nullableString(),
		"confidence":        nullableStringEnum("high", "medium", "low"),
		"location": nullableStrictObject(schema{
			"path": schema{"type": "string"},
			"line": nullableInteger(schema{"minimum": 1}),
		}, []string{"path"}, strict),
		"evidence":      nullableString(),
		"impact":        nullableString(),
		"reproducer":    nullableString(),
		"suggested_fix": nullableString(),
		"validation":    nullableString(),
		"related_spec":  nullableString(),
		"review_pass":   nullableString(),
		"status":        nullableStringEnum("open", "fixed", "accepted_risk", "superseded"),
		"summary":       nullableString(),
	}, []string{"id", "severity", "blocks_completion"}, strict)
}

func attackLogSchema(strict bool) schema {
	return strictObject(schema{
		"target": schema{"type": "string"},
		"attack": schema{"type": "string"},
		"result": stringEnum("finding", "clean", "skipped"),
		"notes":  nullableString(),
	}, []string{"target", "attack", "result"}, strict)
}

func budgetSchema(strict bool) schema {
	return strictObject(schema{
		"max_findings":         nullableInteger(nil),
		"min_attack_angles":    nullableInteger(nil),
		"actual_findings":      nullableInteger(nil),
		"actual_attack_angles": nullableInteger(nil),
		"depth":                nullableString(),
	}, nil, strict)
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

func nullableStrictObject(properties schema, required []string, strict bool) schema {
	out := strictObject(properties, required, strict)
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
