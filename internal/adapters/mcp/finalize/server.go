// Package finalize exposes the host-facing finalize MCP transport.
package finalize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/platform/mcpsubmit"
)

// Options configures the host finalize MCP transport shim.
type Options struct {
	ScafldBinary string
	CWD          string
	OutPath      string
}

// Run serves finalize over MCP stdio. The adapter is transport-only: it
// invokes the scafld finalize CLI child process and never imports the finalize app.
func Run(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer, opts Options) error {
	outPath := strings.TrimSpace(opts.OutPath)
	cleanup := func() {}
	if outPath == "" {
		file, err := os.CreateTemp("", "scafld-finalize-*.json")
		if err != nil {
			return fmt.Errorf("create finalize output file: %w", err)
		}
		outPath = file.Name()
		_ = file.Close()
		cleanup = func() { _ = os.Remove(outPath) }
	}
	defer cleanup()
	return mcpsubmit.Run(ctx, stdin, stdout, stderr, mcpsubmit.Options{
		OutPath:            outPath,
		ServerName:         "scafld",
		ToolName:           "finalize",
		ToolTitle:          "Finalize work",
		ToolDescription:    "Finalize scafld work by running acceptance, external review, and returning blockers or a signed receipt.",
		SchemaJSON:         `{"type":"object","additionalProperties":false,"required":["task_id"],"properties":{"task_id":{"type":"string","description":"Stable scafld task/spec id for the receipt."},"root":{"type":"string","description":"Repository root. Defaults to the MCP server working directory."},"base_ref":{"type":"string","description":"Optional base ref or SHA for a base-delta receipt, such as a PR base SHA or origin/main."},"scope_hint":{"type":"array","items":{"type":"string"},"description":"Explicit changed paths when no hand-authored spec exists."}}}`,
		AllowRepeatedCalls: true,
		ParseAndEncode: func(text string) (mcpsubmit.Accepted, error) {
			return callFinalizeCLI(ctx, opts, text)
		},
	})
}

func callFinalizeCLI(ctx context.Context, opts Options, payload string) (mcpsubmit.Accepted, error) {
	binary := strings.TrimSpace(opts.ScafldBinary)
	if binary == "" {
		binary = "scafld"
	}
	cmd := exec.CommandContext(ctx, binary, "finalize", "--json", "--stdin")
	if strings.TrimSpace(opts.CWD) != "" {
		cmd.Dir = opts.CWD
	}
	cmd.Stdin = strings.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return mcpsubmit.Accepted{}, fmt.Errorf("scafld finalize --json failed: %s", detail)
	}
	data := bytes.TrimSpace(stdout.Bytes())
	if len(data) == 0 {
		return mcpsubmit.Accepted{}, fmt.Errorf("scafld finalize --json returned empty stdout")
	}
	var structured map[string]any
	if err := json.Unmarshal(data, &structured); err != nil {
		return mcpsubmit.Accepted{}, fmt.Errorf("parse scafld finalize --json output: %w", err)
	}
	text := "finalize completed."
	if command, ok := structured["command"].(string); ok && command != "" {
		text = filepath.Base(binary) + " " + command + " completed."
	}
	return mcpsubmit.Accepted{Data: data, Text: text, StructuredContent: structured}, nil
}
