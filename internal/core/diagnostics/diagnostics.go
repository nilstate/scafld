package diagnostics

import (
	"errors"
	"strings"
)

// Path returns the provider diagnostic path attached to err, when one exists.
func Path(err error) string {
	var withDiagnostic interface{ DiagnosticPath() string }
	if errors.As(err, &withDiagnostic) {
		return strings.TrimSpace(withDiagnostic.DiagnosticPath())
	}
	return ""
}

// FailureReason returns a compact one-line control-state reason plus the full
// diagnostic artifact path. The artifact, not the ledger/spec summary, carries
// raw provider transcripts.
func FailureReason(prefix string, err error, maxDetail int) (string, string) {
	detail := ""
	if err != nil {
		detail = strings.TrimSpace(err.Error())
	}
	path := Path(err)
	if path != "" {
		if idx := strings.Index(detail, " (diagnostic: "); idx >= 0 {
			detail = strings.TrimSpace(detail[:idx])
		}
	}
	detail = CompactOneLine(detail, maxDetail)
	if detail == "" {
		if path == "" {
			return prefix, path
		}
		return prefix + " (diagnostic: " + path + ")", path
	}
	if path == "" {
		return prefix + ": " + detail, path
	}
	return prefix + ": " + detail + " (diagnostic: " + path + ")", path
}

// CompactOneLine collapses incidental output formatting for status fields.
func CompactOneLine(text string, max int) string {
	text = strings.Join(strings.Fields(text), " ")
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}
