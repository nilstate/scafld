package mcpsubmit

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestSingleUseDefaultRejectsSecondCall(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	outPath := filepath.Join(t.TempDir(), "submission.json")
	err := Run(context.Background(), strings.NewReader(mcpCalls("submit", 1, 2)), &stdout, &bytes.Buffer{}, Options{
		OutPath:    outPath,
		ToolName:   "submit",
		SchemaJSON: `{"type":"object"}`,
		ParseAndEncode: func(text string) (Accepted, error) {
			return Accepted{Data: []byte(text), Text: "accepted"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(stdout.String(), `"isError":false`); count != 1 {
		t.Fatalf("successful calls = %d\n%s", count, stdout.String())
	}
	if !strings.Contains(stdout.String(), "was already called") {
		t.Fatalf("second call was not rejected:\n%s", stdout.String())
	}
}

func TestRepeatedCallsOptionAllowsSecondCall(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	outPath := filepath.Join(t.TempDir(), "submission.json")
	err := Run(context.Background(), strings.NewReader(mcpCalls("submit", 1, 2)), &stdout, &bytes.Buffer{}, Options{
		OutPath:            outPath,
		ToolName:           "submit",
		SchemaJSON:         `{"type":"object"}`,
		AllowRepeatedCalls: true,
		ParseAndEncode: func(text string) (Accepted, error) {
			return Accepted{Data: []byte(text), Text: "accepted"}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(stdout.String(), `"isError":false`); count != 2 {
		t.Fatalf("successful calls = %d\n%s", count, stdout.String())
	}
}

func mcpCalls(tool string, ids ...int) string {
	var b strings.Builder
	for _, id := range ids {
		data, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      tool,
				"arguments": map[string]any{"id": id},
			},
		})
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String()
}
