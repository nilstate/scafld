package lifecycle

import (
	"context"
	"testing"
)

func TestTerminalEvidenceContextDetachesFromCallerCancellation(t *testing.T) {
	t.Parallel()

	parent, cancelParent := context.WithCancel(context.Background())
	cancelParent()

	ctx, cancel := TerminalEvidenceContext(parent)
	defer cancel()

	if err := ctx.Err(); err != nil {
		t.Fatalf("terminal evidence context should start usable after parent cancellation: %v", err)
	}
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("terminal evidence context should be bounded")
	}
}
