// Package completion evaluates the review authority that allows a task to close.
package completion

import (
	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
	"github.com/nilstate/scafld/v2/internal/core/session"
)

// Authority describes the review evidence that authorizes completion.
type Authority = reviewgate.Authority

// CurrentReviewGate validates the latest review gate that can currently
// authorize completion.
func CurrentReviewGate(ledger session.Session) Authority {
	return reviewgate.CurrentReviewGate(ledger)
}

// TerminalAuthority validates the review gate that authorized the latest
// complete event in an archived task ledger.
func TerminalAuthority(ledger session.Session) Authority {
	return reviewgate.TerminalAuthority(ledger)
}
