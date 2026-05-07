package review

import "strings"

// SplitScope parses comma-separated review scope paths.
func SplitScope(value string) []string {
	var parts []string
	for _, part := range strings.Split(value, ",") {
		text := strings.TrimSpace(part)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return parts
}
