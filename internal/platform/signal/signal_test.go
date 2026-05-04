package signal

import (
	"context"
	"testing"
	"time"
)

func TestSignalContextCancel(t *testing.T) {
	ctx, handler := RootContext(context.Background())
	handler.Stop()
	select {
	case <-ctx.Done():
	default:
		t.Fatal("context not cancelled on stop")
	}
}

func TestSignalInterruptTerminateEscalateContract(t *testing.T) {
	escalated := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	handler := &Handler{}
	handler.record(cancel, Options{Escalate: func() { escalated <- struct{}{} }})
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("first interrupt did not cancel context")
	}
	handler.record(cancel, Options{Escalate: func() { escalated <- struct{}{} }})
	select {
	case <-escalated:
	case <-time.After(time.Second):
		t.Fatal("second interrupt did not escalate")
	}
	if handler.Interrupts.Load() < 2 || !handler.Escalated() {
		t.Fatalf("interrupts=%d escalated=%v", handler.Interrupts.Load(), handler.Escalated())
	}
}
