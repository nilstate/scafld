package hardensubmit

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunListsSubmitHardenToolAndWritesValidDossier(t *testing.T) {
	t.Parallel()

	outPath := filepath.Join(t.TempDir(), "dossier.json")
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"submit_harden","arguments":{"summary":"clean","shape":{"decision":"keep","true_shape":"Shared harden contract.","minimal_plan":"Submit shape and observations.","shared_owner":"internal/core/harden","adapter_boundaries":["MCP accepts the schema","app derives the verdict"],"required_spec_edits":[]},"observations":[{"dimension":"design","result":"clean","anchor":"spec_gap:Summary"},{"dimension":"scope","result":"clean","anchor":"spec_gap:Scope"},{"dimension":"path","result":"clean","anchor":"spec_gap:Scope"},{"dimension":"command","result":"clean","anchor":"spec_gap:Acceptance"},{"dimension":"timing","result":"clean","anchor":"spec_gap:Phases"},{"dimension":"rollback","result":"n/a","anchor":"spec_gap:Rollback"}]}}}`,
		``,
	}, "\n")
	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(input), &stdout, &stderr, outPath); err != nil {
		t.Fatalf("Run err = %v stderr=%s", err, stderr.String())
	}
	lines := nonEmptyLines(stdout.String())
	if len(lines) != 3 {
		t.Fatalf("responses = %d\n%s", len(lines), stdout.String())
	}
	var tools struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &tools); err != nil {
		t.Fatal(err)
	}
	if len(tools.Result.Tools) != 1 || tools.Result.Tools[0].Name != "submit_harden" || tools.Result.Tools[0].InputSchema["additionalProperties"] != false {
		t.Fatalf("tools/list = %s", lines[1])
	}
	if !strings.Contains(tools.Result.Tools[0].Description, "draft as a hypothesis") ||
		!strings.Contains(tools.Result.Tools[0].Description, "reuse-existing-behavior") ||
		!strings.Contains(tools.Result.Tools[0].Description, "advisory feedback") {
		t.Fatalf("tool description = %q", tools.Result.Tools[0].Description)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"verdict"`) || strings.Contains(string(data), `"checks"`) || !strings.Contains(string(data), `"shape"`) || !strings.Contains(string(data), `"observations"`) {
		t.Fatalf("submission = %s", data)
	}
}

func nonEmptyLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
