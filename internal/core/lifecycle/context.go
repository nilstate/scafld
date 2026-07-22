package lifecycle

import (
	"context"
	"time"
)

// TerminalEvidenceTimeout bounds best-effort repair writes after an external
// provider returns or the caller context is already canceled.
const TerminalEvidenceTimeout = 30 * time.Second

// TerminalEvidenceContext returns a short-lived context for recording terminal
// gate evidence. It intentionally detaches from caller cancellation so a timed
// out provider attempt does not strand the task in an in-progress state.
func TerminalEvidenceContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(parent), TerminalEvidenceTimeout)
}
