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
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"submit_harden","arguments":{"summary":"clean","observations":[{"dimension":"design","result":"clean","anchor":"spec_gap:Summary"},{"dimension":"scope","result":"clean","anchor":"spec_gap:Scope"},{"dimension":"path","result":"clean","anchor":"spec_gap:Scope"},{"dimension":"command","result":"clean","anchor":"spec_gap:Acceptance"},{"dimension":"timing","result":"clean","anchor":"spec_gap:Phases"},{"dimension":"rollback","result":"n/a","anchor":"spec_gap:Rollback"}]}}}`,
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
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"verdict"`) || strings.Contains(string(data), `"checks"`) || !strings.Contains(string(data), `"observations"`) {
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
