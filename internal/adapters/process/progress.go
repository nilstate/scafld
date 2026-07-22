package process

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/execution"
)

const defaultProgressInterval = 15 * time.Second

type progressReporter struct {
	mu            sync.Mutex
	out           io.Writer
	label         string
	printedEvents map[string]bool
}

func newProgressReporter(out io.Writer, label string) *progressReporter {
	if strings.TrimSpace(label) == "" {
		label = "process"
	}
	return &progressReporter{out: out, label: strings.TrimSpace(label), printedEvents: map[string]bool{}}
}

func (p *progressReporter) line(format string, args ...interface{}) {
	if p == nil || p.out == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "scafld %s %s\n", p.label, fmt.Sprintf(format, args...))
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
	if p == nil || p.out == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.printedEvents[event] {
		return
	}
	p.printedEvents[event] = true
	fmt.Fprintf(p.out, "scafld %s event %s\n", p.label, event)
}

func (p *progressReporter) running(started time.Time, lastActivity time.Time, lastProgress *time.Time, interval time.Duration) {
	if p == nil || p.out == nil || interval <= 0 || lastProgress == nil {
		return
	}
	now := time.Now()
	if now.Sub(*lastProgress) < interval {
		return
	}
	*lastProgress = now
	elapsed := now.Sub(started).Round(time.Second)
	since := now.Sub(lastActivity).Round(time.Second)
	p.line("running elapsed=%s last_output=%s", elapsed, since)
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
