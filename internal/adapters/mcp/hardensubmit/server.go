// Package hardensubmit exposes the hidden MCP submit_harden transport.
package hardensubmit

import (
	"context"
	"io"

	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
	"github.com/nilstate/scafld/v2/internal/platform/mcpsubmit"
)

// Run serves a minimal MCP stdio server that accepts exactly one
// submit_harden tool call and writes canonical HardenDossier JSON to outPath.
func Run(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer, outPath string) error {
	return mcpsubmit.Run(ctx, stdin, stdout, stderr, mcpsubmit.Options{
		OutPath:         outPath,
		ServerName:      "scafld-harden-submit",
		ToolName:        "submit_harden",
		ToolTitle:       "Submit scafld hardening",
		ToolDescription: "Submit the final scafld HardenDossier. Call exactly once after stress-testing the draft spec.",
		SchemaJSON:      coreharden.DossierSchemaJSON(),
		ParseAndEncode: func(text string) (mcpsubmit.Accepted, error) {
			dossier, err := coreharden.ParseText(text)
			if err != nil {
				return mcpsubmit.Accepted{}, err
			}
			return mcpsubmit.Accepted{
				Data: []byte(coreharden.EncodeDossier(dossier)),
				Text: "HardenDossier accepted.",
				StructuredContent: map[string]any{
					"ok":      true,
					"verdict": dossier.Verdict,
				},
			}, nil
		},
	})
}
