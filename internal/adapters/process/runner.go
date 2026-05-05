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

const defaultMaxCaptureBytes = 8 * 1024 * 1024

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
	cmd.Env = append(os.Environ(), req.Env...)
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
	go pump(&pumps, stdout, state, "stdout", progress, req.StdoutEventInspector)
	go pump(&pumps, stderr, state, "stderr", progress, nil)
	waitCh := make(chan error, 1)
	go func() {
		pumps.Wait()
		waitCh <- cmd.Wait()
	}()
	result, waitErr := waitProcess(ctx, cmd, waitCh, state, req)
	result.Stdout, result.Stderr, result.Output, result.DroppedBytes, result.StdoutEvents = state.snapshot()
	progress.result(result, waitErr)
	if r.DiagnosticsDir != "" {
		if path, writeErr := r.writeDiagnostic(displayCommand, result); writeErr == nil {
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

type capture struct {
	mu       sync.Mutex
	stdout   slidingBuffer
	stderr   slidingBuffer
	combined slidingBuffer
	last     time.Time
	events   map[string]int
}

func newCapture(maxBytes int) *capture {
	return &capture{
		stdout:   newSlidingBuffer(maxBytes),
		stderr:   newSlidingBuffer(maxBytes),
		combined: newSlidingBuffer(maxBytes),
		last:     time.Now(),
		events:   map[string]int{},
	}
}

func (c *capture) touch() {
	c.mu.Lock()
	c.last = time.Now()
	c.mu.Unlock()
}

func (c *capture) write(stream string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch stream {
	case "stdout":
		c.stdout.Write(data)
	case "stderr":
		c.stderr.Write(data)
	}
	c.combined.Write(data)
	c.last = time.Now()
}

func (c *capture) recordEvent(event string) {
	c.mu.Lock()
	c.events[event]++
	c.mu.Unlock()
}

func (c *capture) snapshot() (string, string, string, int, map[string]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	events := make(map[string]int, len(c.events))
	for key, value := range c.events {
		events[key] = value
	}
	dropped := c.stdout.Dropped() + c.stderr.Dropped() + c.combined.Dropped()
	return c.stdout.String(), c.stderr.String(), c.combined.String(), dropped, events
}

func (c *capture) lastActivity() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last
}

func pump(wg *sync.WaitGroup, reader io.Reader, state *capture, stream string, progress *progressReporter, inspector func(string) string) {
	defer wg.Done()
	buf := make([]byte, 32*1024)
	var lineBuffer strings.Builder
	lineMode := inspector != nil || progress != nil
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			state.write(stream, chunk)
			if lineMode {
				dispatchLines(&lineBuffer, string(chunk), stream, inspector, state, progress)
			}
		}
		if err != nil {
			if lineMode && lineBuffer.Len() > 0 {
				handleLine(lineBuffer.String(), stream, inspector, state, progress)
			}
			return
		}
	}
}

func dispatchLines(lineBuffer *strings.Builder, chunk string, stream string, inspector func(string) string, state *capture, progress *progressReporter) {
	for _, part := range strings.SplitAfter(chunk, "\n") {
		if strings.HasSuffix(part, "\n") {
			lineBuffer.WriteString(strings.TrimSuffix(part, "\n"))
			line := lineBuffer.String()
			lineBuffer.Reset()
			handleLine(line, stream, inspector, state, progress)
			continue
		}
		lineBuffer.WriteString(part)
	}
}

func handleLine(line string, stream string, inspector func(string) string, state *capture, progress *progressReporter) {
	if inspector != nil {
		if event := inspector(line); event != "" {
			state.recordEvent(event)
			progress.event(event)
		}
	}
	if stream == "stderr" {
		progress.stream(stream, line)
	}
}

func waitProcess(ctx context.Context, cmd *exec.Cmd, waitCh <-chan error, state *capture, req execution.Request) (execution.Result, error) {
	started := time.Now()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	absolute := time.NewTimer(durationOrNever(req.Timeout))
	defer absolute.Stop()
	for {
		select {
		case err := <-waitCh:
			return processResult(cmd, "", false, started, state, req), normalizeExit(err)
		case <-ctx.Done():
			result := terminate(cmd, waitCh, "cancelled", req.TerminateGrace)
			result = enrichResult(result, started, state, req)
			return result, ctx.Err()
		case <-absolute.C:
			result := terminate(cmd, waitCh, "absolute_timeout", req.TerminateGrace)
			result.TimedOut = true
			result = enrichResult(result, started, state, req)
			return result, ErrTimeout
		case <-ticker.C:
			if req.IdleTimeout > 0 && time.Since(state.lastActivity()) > req.IdleTimeout {
				result := terminate(cmd, waitCh, "idle_timeout", req.TerminateGrace)
				result.TimedOut = true
				result = enrichResult(result, started, state, req)
				return result, ErrIdle
			}
		}
	}
}

func terminate(cmd *exec.Cmd, waitCh <-chan error, reason string, grace time.Duration) execution.Result {
	if grace <= 0 {
		grace = 250 * time.Millisecond
	}
	_ = terminateProcessGroup(cmd)
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case <-waitCh:
	case <-timer.C:
		_ = killProcessGroup(cmd)
		<-waitCh
	}
	exit := -1
	if cmd != nil && cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}
	return execution.Result{ExitCode: exit, KillReason: reason, TimedOut: true}
}

func processResult(cmd *exec.Cmd, killReason string, timedOut bool, started time.Time, state *capture, req execution.Request) execution.Result {
	exit := 0
	if cmd == nil || cmd.ProcessState == nil {
		exit = -1
	} else {
		exit = cmd.ProcessState.ExitCode()
	}
	return enrichResult(execution.Result{ExitCode: exit, KillReason: killReason, TimedOut: timedOut}, started, state, req)
}

func enrichResult(result execution.Result, started time.Time, state *capture, req execution.Request) execution.Result {
	result.WallElapsed = time.Since(started)
	result.TimeSinceLastByte = time.Since(state.lastActivity())
	result.IdleTimeout = req.IdleTimeout
	result.AbsoluteTimeout = req.Timeout
	return result
}

func normalizeExit(err error) error {
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil
	}
	return err
}

func durationOrNever(duration time.Duration) time.Duration {
	if duration <= 0 {
		return 100 * 365 * 24 * time.Hour
	}
	return duration
}

func maxCaptureBytes(value int) int {
	if value <= 0 {
		return defaultMaxCaptureBytes
	}
	return value
}

type progressReporter struct {
	mu    sync.Mutex
	out   io.Writer
	label string
}

func newProgressReporter(out io.Writer, label string) *progressReporter {
	if strings.TrimSpace(label) == "" {
		label = "process"
	}
	return &progressReporter{out: out, label: strings.TrimSpace(label)}
}

func (p *progressReporter) line(format string, args ...interface{}) {
	if p == nil || p.out == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "scafld %s: %s\n", p.label, fmt.Sprintf(format, args...))
}

func (p *progressReporter) stream(stream string, line string) {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return
	}
	p.line("%s %s", stream, line)
}

func (p *progressReporter) event(event string) {
	if strings.TrimSpace(event) == "" {
		return
	}
	p.line("event %s", event)
}

func (p *progressReporter) result(result execution.Result, err error) {
	elapsed := result.WallElapsed.Round(time.Millisecond)
	since := result.TimeSinceLastByte.Round(time.Millisecond)
	if err != nil || result.KillReason != "" {
		reason := result.KillReason
		if reason == "" {
			reason = err.Error()
		}
		p.line("stopped reason=%s exit=%d elapsed=%s last_output=%s", reason, result.ExitCode, elapsed, since)
		return
	}
	p.line("completed exit=%d elapsed=%s last_output=%s", result.ExitCode, elapsed, since)
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

func (r Runner) writeDiagnostic(command string, result execution.Result) (string, error) {
	if err := os.MkdirAll(r.DiagnosticsDir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("command-%d.txt", time.Now().UnixNano())
	path := filepath.Join(r.DiagnosticsDir, name)
	data := []byte(
		"command: " + command +
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

type slidingBuffer struct {
	data    []byte
	max     int
	dropped int
}

func newSlidingBuffer(maxBytes int) slidingBuffer {
	return slidingBuffer{max: maxBytes}
}

func (b *slidingBuffer) Write(data []byte) {
	b.data = append(b.data, data...)
	if b.max > 0 && len(b.data) > b.max {
		excess := len(b.data) - b.max
		copy(b.data, b.data[excess:])
		b.data = b.data[:b.max]
		b.dropped += excess
	}
}

func (b *slidingBuffer) String() string {
	body := string(b.data)
	if b.dropped > 0 {
		return fmt.Sprintf("[truncated %d earlier bytes]\n%s", b.dropped, body)
	}
	return body
}

func (b *slidingBuffer) Dropped() int {
	return b.dropped
}
