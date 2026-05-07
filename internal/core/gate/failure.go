package gate

import "fmt"

// Failure is the machine-readable repair contract for a blocked gate.
type Failure struct {
	Gate     string   `json:"gate"`
	Status   string   `json:"status,omitempty"`
	Reason   string   `json:"reason"`
	Evidence []string `json:"evidence,omitempty"`
	Expected string   `json:"expected,omitempty"`
	Actual   string   `json:"actual,omitempty"`
	Blockers []string `json:"blockers,omitempty"`
	Next     string   `json:"next,omitempty"`
}

// Error wraps a cause with a deterministic gate failure payload.
type Error struct {
	Cause   error
	Failure Failure
}

// New returns an error that can be matched with errors.Is/As and rendered by
// automation-facing adapters.
func New(cause error, failure Failure) Error {
	return Error{Cause: cause, Failure: failure}
}

// Error returns the human-readable cause plus gate reason.
func (e Error) Error() string {
	if e.Cause == nil {
		return e.Failure.Reason
	}
	if e.Failure.Reason == "" {
		return e.Cause.Error()
	}
	return fmt.Sprintf("%s: %s", e.Cause, e.Failure.Reason)
}

// Unwrap exposes the underlying cause for errors.Is.
func (e Error) Unwrap() error {
	return e.Cause
}
