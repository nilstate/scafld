// Package hostgate exposes the host-facing scafld_gate MCP transport.
package hostgate

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

// Options configures the host gate MCP transport shim.
type Options struct {
	ScafldBinary string
	CWD          string
	OutPath      string
}

// Run serves scafld_gate over MCP stdio. The adapter is transport-only: it
// invokes the scafld gate CLI child process and never imports the gate app.
func Run(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer, opts Options) error {
	outPath := strings.TrimSpace(opts.OutPath)
	cleanup := func() {}
	if outPath == "" {
		file, err := os.CreateTemp("", "scafld-hostgate-*.json")
		if err != nil {
			return fmt.Errorf("create hostgate output file: %w", err)
		}
		outPath = file.Name()
		_ = file.Close()
		cleanup = func() { _ = os.Remove(outPath) }
	}
	defer cleanup()
	return mcpsubmit.Run(ctx, stdin, stdout, stderr, mcpsubmit.Options{
		OutPath:            outPath,
		ServerName:         "scafld-host-gate",
		ToolName:           "scafld_gate",
		ToolTitle:          "Run scafld gate",
		ToolDescription:    "Run the scafld accountability gate and return findings or a signed receipt.",
		SchemaJSON:         `{"type":"object","additionalProperties":true}`,
		AllowRepeatedCalls: true,
		ParseAndEncode: func(text string) (mcpsubmit.Accepted, error) {
			return callGateCLI(ctx, opts, text)
		},
	})
}

func callGateCLI(ctx context.Context, opts Options, payload string) (mcpsubmit.Accepted, error) {
	binary := strings.TrimSpace(opts.ScafldBinary)
	if binary == "" {
		binary = "scafld"
	}
	cmd := exec.CommandContext(ctx, binary, "gate", "--json", "--stdin")
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
		return mcpsubmit.Accepted{}, fmt.Errorf("scafld gate --json failed: %s", detail)
	}
	data := bytes.TrimSpace(stdout.Bytes())
	if len(data) == 0 {
		return mcpsubmit.Accepted{}, fmt.Errorf("scafld gate --json returned empty stdout")
	}
	var structured map[string]any
	if err := json.Unmarshal(data, &structured); err != nil {
		return mcpsubmit.Accepted{}, fmt.Errorf("parse scafld gate --json output: %w", err)
	}
	text := "scafld_gate completed."
	if command, ok := structured["command"].(string); ok && command != "" {
		text = filepath.Base(binary) + " " + command + " completed."
	}
	return mcpsubmit.Accepted{Data: data, Text: text, StructuredContent: structured}, nil
}
