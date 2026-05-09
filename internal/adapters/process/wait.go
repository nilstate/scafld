package process

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
)

func waitProcess(ctx context.Context, cmd *exec.Cmd, waitCh <-chan error, state *capture, req execution.Request, progress *progressReporter) (execution.Result, error) {
	started := time.Now()
	lastProgress := started
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
			progress.running(started, state.lastActivity(), &lastProgress, progressInterval(req.ProgressInterval))
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

func progressInterval(value time.Duration) time.Duration {
	if value < 0 {
		return 0
	}
	if value == 0 {
		return defaultProgressInterval
	}
	return value
}
