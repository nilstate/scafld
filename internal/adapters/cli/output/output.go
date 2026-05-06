// Package output formats human-readable command output for the CLI adapter.
package output

import (
	"fmt"
	"strings"

	appreview "github.com/nilstate/scafld/v2/internal/app/review"
	appstatus "github.com/nilstate/scafld/v2/internal/app/status"
)

// Review formats the review gate result so findings are visible in the normal path.
func Review(out appreview.Output) string {
	var b strings.Builder
	fmt.Fprintf(&b, "review verdict: %s\n", out.Verdict)
	if len(out.Findings) > 0 {
		fmt.Fprintf(&b, "findings:\n")
		for _, finding := range out.Findings {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", finding.Severity, finding.ID, finding.Summary)
		}
	}
	if out.Next != "" {
		fmt.Fprintf(&b, "next: %s\n", out.Next)
	}
	return b.String()
}

// Status formats status output with the latest review findings when present.
func Status(out appstatus.Output) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\nnext: %s\n", out.TaskID, out.Status, out.Next)
	if out.Review.Running {
		fmt.Fprintf(&b, "review: running\n")
		if out.Review.Reason != "" {
			fmt.Fprintf(&b, "reason: %s\n", out.Review.Reason)
		}
	} else if out.Review.Verdict != "" {
		fmt.Fprintf(&b, "review: %s\n", out.Review.Verdict)
		for _, finding := range out.Review.Findings {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", finding.Severity, finding.ID, finding.Summary)
		}
	} else if out.Review.AttemptStatus != "" {
		fmt.Fprintf(&b, "review: %s\n", out.Review.AttemptStatus)
		if out.Review.Reason != "" {
			fmt.Fprintf(&b, "reason: %s\n", out.Review.Reason)
		}
	}
	return b.String()
}
