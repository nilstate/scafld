package process

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
)

func TestCommandWritesDiagnostic(t *testing.T) {
	t.Parallel()
	result, err := (Runner{DiagnosticsDir: t.TempDir()}).Run(context.Background(), execution.Request{Command: "printf ok", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || result.Output != "ok" || result.DiagnosticPath == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCommandSplitsStdoutAndStderr(t *testing.T) {
	t.Parallel()
	result, err := (Runner{}).Run(context.Background(), execution.Request{Command: "printf out; printf err >&2", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "out" || result.Stderr != "err" {
		t.Fatalf("stdout/stderr not split: %+v", result)
	}
}

func TestCommandReceivesStdin(t *testing.T) {
	t.Parallel()
	result, err := (Runner{}).Run(context.Background(), execution.Request{Command: "cat", Input: "prompt", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "prompt" {
		t.Fatalf("stdin not sent to process: %+v", result)
	}
}

func TestCommandAcceptsArgv(t *testing.T) {
	t.Parallel()
	result, err := (Runner{}).Run(context.Background(), execution.Request{Args: []string{"printf", "argv"}, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stdout != "argv" {
		t.Fatalf("argv execution failed: %+v", result)
	}
}

func TestCommandCapsOutputAndRecordsEvents(t *testing.T) {
	t.Parallel()
	result, err := (Runner{}).Run(context.Background(), execution.Request{
		Command:         `printf '{"type":"system"}\n{"type":"result","subtype":"success"}\nabcdef'`,
		Timeout:         time.Second,
		MaxCaptureBytes: 4,
		StdoutEventInspector: func(line string) string {
			if strings.Contains(line, "result") {
				return "result.success"
			}
			if strings.Contains(line, "system") {
				return "system"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.DroppedBytes == 0 || !strings.Contains(result.Stdout, "cdef") {
		t.Fatalf("output should be capped and keep tail: %+v", result)
	}
	if result.StdoutEvents["system"] != 1 || result.StdoutEvents["result.success"] != 1 {
		t.Fatalf("events = %+v", result.StdoutEvents)
	}
}

func TestCommandStreamsProgressToReporter(t *testing.T) {
	t.Parallel()

	var progress bytes.Buffer
	result, err := (Runner{Progress: &progress, ProgressLabel: "review provider"}).Run(context.Background(), execution.Request{
		Command: `printf '{"type":"result","subtype":"success"}\n'; printf 'thinking\n' >&2`,
		Timeout: time.Second,
		StdoutEventInspector: func(line string) string {
			if strings.Contains(line, "result") {
				return "result.success"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.StdoutEvents["result.success"] != 1 {
		t.Fatalf("events = %+v", result.StdoutEvents)
	}
	body := progress.String()
	for _, want := range []string{
		"scafld review provider started",
		"scafld review provider event result.success",
		"scafld review provider stderr thinking",
		"scafld review provider completed exit=0",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("progress missing %q:\n%s", want, body)
		}
	}
}

func TestCommandSuppressesRawStderrProgressAndReportsHeartbeat(t *testing.T) {
	t.Parallel()

	var progress bytes.Buffer
	result, err := (Runner{Progress: &progress, ProgressLabel: "review provider"}).Run(context.Background(), execution.Request{
		Command:                `printf "$SCAFLD_NOISY_TOKEN\n" >&2; sleep 0.08`,
		Env:                    []string{"SCAFLD_NOISY_TOKEN=investigating"},
		Timeout:                time.Second,
		SuppressProgressStderr: true,
		ProgressInterval:       20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Stderr, "investigating") {
		t.Fatalf("raw stderr should remain captured in diagnostics/result: %+v", result)
	}
	body := progress.String()
	if strings.Contains(body, "investigating") {
		t.Fatalf("raw provider stderr leaked into progress:\n%s", body)
	}
	for _, want := range []string{
		"scafld review provider started",
		"scafld review provider running elapsed=",
		"scafld review provider completed exit=0",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("progress missing %q:\n%s", want, body)
		}
	}
}

func TestCommandAbsoluteTimeout(t *testing.T) {
	t.Parallel()
	_, err := (Runner{}).Run(context.Background(), execution.Request{Command: "sleep 1", Timeout: time.Millisecond})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestCommandIdleTimeout(t *testing.T) {
	t.Parallel()
	idle, err := (Runner{}).Run(context.Background(), execution.Request{Command: "printf start; sleep 1", Timeout: time.Second, IdleTimeout: time.Millisecond})
	if !errors.Is(err, ErrIdle) || idle.KillReason != "idle_timeout" {
		t.Fatalf("idle result=%+v err=%v", idle, err)
	}
}

func TestCommandDoesNotSpawnWhenPreCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "should-not-exist")
	_, err := (Runner{}).Run(ctx, execution.Request{Command: "touch " + path})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("pre-cancelled command should not spawn, stat err = %v", statErr)
	}
}

func TestSignalInterruptTerminateEscalateScenario(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (Runner{}).Run(ctx, execution.Request{Command: "sleep 1"})
	if err == nil {
		t.Fatal("expected cancellation")
	}
}
