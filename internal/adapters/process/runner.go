package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
)

var (
	// ErrTimeout wraps absolute process timeout failures.
	ErrTimeout = errors.New("process timeout")
	// ErrIdle wraps idle timeout failures.
	ErrIdle = errors.New("process idle timeout")
)

// Runner executes external commands and writes optional diagnostics.
type Runner struct {
	DiagnosticsDir string
	Progress       io.Writer
	ProgressLabel  string
}

// Run executes req with streaming stdout/stderr capture and watchdogs.
func (r Runner) Run(ctx context.Context, req execution.Request) (execution.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return execution.Result{ExitCode: -1, KillReason: "cancelled"}, err
	}
	cmd, displayCommand, err := commandForRequest(req)
	if err != nil {
		return execution.Result{}, err
	}
	if req.EnvMode == execution.EnvModeExact {
		cmd.Env = append(make([]string, 0, len(req.Env)), req.Env...)
	} else {
		cmd.Env = append(os.Environ(), req.Env...)
	}
	configureProcessGroup(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return execution.Result{}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return execution.Result{}, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return execution.Result{}, fmt.Errorf("start command: %w", err)
	}
	progress := newProgressReporter(r.Progress, r.ProgressLabel)
	progress.line("started %s", summarizeCommand(displayCommand))
	state := newCapture(maxCaptureBytes(req.MaxCaptureBytes))
	state.touch()
	var pumps sync.WaitGroup
	pumps.Add(2)
	go pump(&pumps, stdout, state, "stdout", progress, req.StdoutEventInspector, req.SuppressProgressStderr)
	go pump(&pumps, stderr, state, "stderr", progress, nil, req.SuppressProgressStderr)
	waitCh := make(chan error, 1)
	go func() {
		pumps.Wait()
		waitCh <- cmd.Wait()
	}()
	result, waitErr := waitProcess(ctx, cmd, waitCh, state, req, progress)
	result.Stdout, result.Stderr, result.Output, result.DroppedBytes, result.StdoutEvents = state.snapshot()
	progress.result(result, waitErr)
	if r.DiagnosticsDir != "" {
		if path, writeErr := r.writeDiagnostic(req, displayCommand, result); writeErr == nil {
			result.DiagnosticPath = path
		}
	}
	return result, waitErr
}

func commandForRequest(req execution.Request) (*exec.Cmd, string, error) {
	if req.Command == "" && len(req.Args) == 0 {
		return nil, "", fmt.Errorf("command or args are required")
	}
	if req.Command != "" && len(req.Args) > 0 {
		return nil, "", fmt.Errorf("command and args are mutually exclusive")
	}
	cmd, displayCommand, err := baseCommand(req)
	if err != nil {
		return nil, "", err
	}
	if req.CWD != "" {
		cmd.Dir = req.CWD
	}
	if req.Input != "" {
		cmd.Stdin = strings.NewReader(req.Input)
	}
	return cmd, displayCommand, nil
}

func baseCommand(req execution.Request) (*exec.Cmd, string, error) {
	if len(req.Args) == 0 {
		return exec.Command("sh", "-c", req.Command), req.Command, nil
	}
	if strings.TrimSpace(req.Args[0]) == "" {
		return nil, "", fmt.Errorf("args[0] is required")
	}
	return exec.Command(req.Args[0], req.Args[1:]...), strings.Join(req.Args, " "), nil
}

func summarizeCommand(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "(empty command)"
	}
	fields[0] = filepath.Base(fields[0])
	summary := strings.Join(fields, " ")
	const max = 240
	if len(summary) > max {
		return summary[:max] + "...[truncated]"
	}
	return summary
}

func (r Runner) writeDiagnostic(req execution.Request, command string, result execution.Result) (string, error) {
	if err := os.MkdirAll(r.DiagnosticsDir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("command-%d.txt", time.Now().UnixNano())
	path := filepath.Join(r.DiagnosticsDir, name)
	cwd := req.CWD
	if cwd == "" {
		cwd = "."
	}
	data := []byte(
		"command: " + command +
			"\ncwd: " + cwd +
			"\nenv_overrides: " + strings.Join(envOverrideNames(req.Env), ",") +
			"\nexit_code: " + fmt.Sprint(result.ExitCode) +
			"\nkill_reason: " + result.KillReason +
			"\nwall_elapsed: " + result.WallElapsed.String() +
			"\ntime_since_last_byte: " + result.TimeSinceLastByte.String() +
			"\nidle_timeout: " + result.IdleTimeout.String() +
			"\nabsolute_timeout: " + result.AbsoluteTimeout.String() +
			"\ndropped_bytes: " + fmt.Sprint(result.DroppedBytes) +
			"\nstdout_events: " + fmt.Sprint(result.StdoutEvents) +
			"\n\nstdout:\n" + result.Stdout +
			"\n\nstderr:\n" + result.Stderr,
	)
	return path, os.WriteFile(path, data, 0o644)
}

func envOverrideNames(env []string) []string {
	names := make([]string, 0, len(env))
	for _, item := range env {
		name, _, ok := strings.Cut(item, "=")
		if !ok {
			name = item
		}
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}
