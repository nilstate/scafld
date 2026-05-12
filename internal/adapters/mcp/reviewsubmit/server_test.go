package reviewsubmit

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunListsSubmitReviewToolAndWritesValidDossier(t *testing.T) {
	t.Parallel()

	outPath := filepath.Join(t.TempDir(), "dossier.json")
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"submit_review","arguments":{"verdict":"pass","mode":"discover","summary":"clean","findings":[],"attack_log":[{"target":"diff","attack":"scan","result":"clean"}],"budget":{"actual_attack_angles":1,"depth":"test"}}}}`,
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
	if len(tools.Result.Tools) != 1 || tools.Result.Tools[0].Name != "submit_review" || tools.Result.Tools[0].InputSchema["additionalProperties"] != false {
		t.Fatalf("tools/list = %s", lines[1])
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"verdict":"pass"`) || !strings.Contains(string(data), `"attack_log"`) {
		t.Fatalf("submission = %s", data)
	}
}

func TestRunRejectsInvalidDossierWithoutWriting(t *testing.T) {
	t.Parallel()

	outPath := filepath.Join(t.TempDir(), "dossier.json")
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"submit_review","arguments":{"verdict":"pass","mode":"discover","summary":"bad","findings":[{"id":"note","severity":"low","blocks_completion":false,"location":"file.go:12","summary":"bad"}],"attack_log":[{"angle":"wrong","result":"clean"}],"budget":{"actual_attack_angles":1}}}}` + "\n"
	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), strings.NewReader(input), &stdout, &stderr, outPath); err != nil {
		t.Fatalf("Run err = %v stderr=%s", err, stderr.String())
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("invalid dossier wrote output, stat err=%v", err)
	}
	if !strings.Contains(stdout.String(), `"isError":true`) || !strings.Contains(stdout.String(), "invalid review dossier") {
		t.Fatalf("stdout = %s", stdout.String())
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
