package review

import (
	"strconv"
	"strings"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
)

// ModeFromFlags resolves CLI mode flags into the provider review mode.
func ModeFromFlags(mode string, verify bool, full bool) corereview.Mode {
	if verify {
		return corereview.ModeVerify
	}
	if full {
		return corereview.ModeDiscover
	}
	if strings.TrimSpace(mode) == string(corereview.ModeVerify) {
		return corereview.ModeVerify
	}
	return corereview.ModeDiscover
}

// PositiveInt parses non-negative integer CLI values. Invalid values use the
// unset value because review budgets are advisory, not gate inputs.
func PositiveInt(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// PositiveOrDefault returns fallback when value is unset.
func PositiveOrDefault(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// FirstNonEmpty returns the first non-blank value.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
