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
		"verdict":    stringEnum(VerdictPass, VerdictNeedsRevision),
		"summary":    schema{"type": "string", "minLength": 1},
		"checks":     schema{"type": "array", "minItems": len(RequiredCheckNames), "items": checkSchema(strict)},
		"issues":     schema{"type": "array", "items": issueSchema(strict)},
		"attack_log": schema{"type": "array", "minItems": 1, "items": attackLogSchema(strict)},
	}, []string{"verdict", "summary", "checks", "issues", "attack_log"}, strict)
	root["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	root["title"] = "scafld HardenDossier provider response"
	return root
}

func checkSchema(strict bool) schema {
	return strictObject(schema{
		"name":        schema{"type": "string", "enum": RequiredCheckNames},
		"grounded_in": schema{"type": "string", "minLength": 1},
		"result":      stringEnum("passed", "failed", "not_applicable"),
		"evidence":    schema{"type": "string", "minLength": 1},
	}, []string{"name", "grounded_in", "result", "evidence"}, strict)
}

func issueSchema(strict bool) schema {
	return strictObject(schema{
		"id":                 nullableString(),
		"kind":               schema{"type": "string", "minLength": 1},
		"severity":           stringEnum("critical", "high", "medium", "low"),
		"blocks_approval":    schema{"type": "boolean"},
		"status":             stringEnum("open", "fixed", "accepted_risk", "superseded"),
		"grounded_in":        schema{"type": "string", "minLength": 1},
		"summary":            schema{"type": "string", "minLength": 1},
		"evidence":           schema{"type": "string", "minLength": 1},
		"recommendation":     schema{"type": "string", "minLength": 1},
		"question":           nullableString(),
		"recommended_answer": nullableString(),
		"if_unanswered":      nullableString(),
	}, []string{"kind", "severity", "blocks_approval", "status", "grounded_in", "summary", "evidence", "recommendation"}, strict)
}

func attackLogSchema(strict bool) schema {
	return strictObject(schema{
		"target": schema{"type": "string", "minLength": 1},
		"attack": schema{"type": "string", "minLength": 1},
		"result": stringEnum("finding", "clean", "skipped"),
		"notes":  nullableString(),
	}, []string{"target", "attack", "result"}, strict)
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

func sortedSchemaKeys(properties schema) []string {
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
