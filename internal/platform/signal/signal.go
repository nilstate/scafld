package signal

import (
	"context"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

// Handler tracks interrupt state and stops signal handling.
type Handler struct {
	Interrupts atomic.Int32
	escalated  atomic.Bool
	stop       func()
}

// Options configures interrupt escalation behavior.
type Options struct {
	Escalate func()
}

// RootContext returns a cancellable root context controlled by OS interrupts.
func RootContext(parent context.Context) (context.Context, *Handler) {
	return RootContextWithOptions(parent, Options{})
}

// RootContextWithOptions returns an interrupt-controlled root context.
func RootContextWithOptions(parent context.Context, opts Options) (context.Context, *Handler) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	handler := &Handler{
		stop: func() {
			signal.Stop(ch)
			close(ch)
			cancel()
		},
	}
	go func() {
		for range ch {
			handler.record(cancel, opts)
		}
	}()
	return ctx, handler
}

func (h *Handler) record(cancel context.CancelFunc, opts Options) {
	if h.Interrupts.Add(1) > 1 {
		h.escalated.Store(true)
		if opts.Escalate != nil {
			opts.Escalate()
		}
	}
	cancel()
}

// Stop unregisters signal handling and cancels the root context.
func (h *Handler) Stop() {
	if h == nil || h.stop == nil {
		return
	}
	h.stop()
	h.stop = nil
}

// Escalated reports whether a second interrupt requested escalation.
func (h *Handler) Escalated() bool {
	return h != nil && h.escalated.Load()
}
