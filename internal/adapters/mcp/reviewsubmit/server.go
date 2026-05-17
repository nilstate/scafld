// Package reviewsubmit exposes the hidden MCP submit_review transport.
package reviewsubmit

import (
	"context"
	"io"

	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/platform/mcpsubmit"
)

// Run serves a minimal MCP stdio server that accepts exactly one
// submit_review tool call and writes canonical ReviewDossier JSON to outPath.
func Run(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer, outPath string) error {
	return mcpsubmit.Run(ctx, stdin, stdout, stderr, mcpsubmit.Options{
		OutPath:         outPath,
		ServerName:      "scafld-review-submit",
		ToolName:        "submit_review",
		ToolTitle:       "Submit scafld review",
		ToolDescription: "Submit the final scafld ReviewDossier. Call exactly once after completing the read-only adversarial review.",
		SchemaJSON:      review.DossierSchemaJSON(),
		ParseAndEncode: func(text string) (mcpsubmit.Accepted, error) {
			dossier, err := review.ParseText(text)
			if err != nil {
				return mcpsubmit.Accepted{}, err
			}
			return mcpsubmit.Accepted{
				Data: []byte(review.EncodeDossier(dossier)),
				Text: "ReviewDossier accepted.",
				StructuredContent: map[string]any{
					"ok":      true,
					"verdict": dossier.Verdict,
				},
			}, nil
		},
	})
}
