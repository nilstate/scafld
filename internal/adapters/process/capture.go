package process

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const defaultMaxCaptureBytes = 8 * 1024 * 1024

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

func pump(wg *sync.WaitGroup, reader io.Reader, state *capture, stream string, progress *progressReporter, inspector func(string) string, suppressProgressStderr bool) {
	defer wg.Done()
	buf := make([]byte, 32*1024)
	var lineBuffer strings.Builder
	lineMode := inspector != nil || (progress != nil && !(stream == "stderr" && suppressProgressStderr))
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
