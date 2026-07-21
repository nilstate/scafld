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
		ToolDescription: "Submit the final scafld HardenDossier. Call exactly once after stress-testing the draft spec. Treat the draft as a hypothesis: if reject/no-op, shrink, reframe, move-owner, or reuse-existing-behavior is materially better, record that in shape instead of softening it into advisory feedback. The shape object must answer keep, shrink, reframe, or reject before observations. The observations array must cover " + coreharden.RequiredDimensionList() + "; fill result, anchor, and note/default/status where applicable. The design observation must challenge shared architecture and keep API/MCP/CLI/provider/docs surfaces as light adapters over common core/app behavior.",
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
					"verdict": coreharden.VerdictFromDossier(dossier),
				},
			}, nil
		},
	})
}
