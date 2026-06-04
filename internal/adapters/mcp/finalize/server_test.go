package finalize

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalizeTransportExposesFinalizeAndAllowsRepeatedCalls(t *testing.T) {
	t.Parallel()

	script := filepath.Join(t.TempDir(), "scafld")
	logPath := filepath.Join(t.TempDir(), "args.log")
	body := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> " + shellQuote(logPath) + "\n" +
		"cat >/dev/null\n" +
		"printf '{\"ok\":true,\"command\":\"finalize\",\"result\":{\"receipt\":\"signed\"}}\\n'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"finalize","arguments":{"task_id":"one"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"finalize","arguments":{"task_id":"two"}}}`,
	}, "\n") + "\n"
	if err := Run(context.Background(), strings.NewReader(input), &stdout, &bytes.Buffer{}, Options{ScafldBinary: script}); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, `"name":"finalize"`) {
		t.Fatalf("tool list missing finalize:\n%s", out)
	}
	if count := strings.Count(out, `"isError":false`); count != 2 {
		t.Fatalf("finalize calls = %d, want 2 repeated successes:\n%s", count, out)
	}
	args, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(args), "finalize --json --stdin"); got != 2 {
		t.Fatalf("child invocations = %d\n%s", got, args)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
