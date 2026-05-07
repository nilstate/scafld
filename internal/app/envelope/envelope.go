package envelope

import "github.com/nilstate/scafld/v2/internal/core/gate"

// NextAction describes the next machine-actionable step for JSON callers.
type NextAction struct {
	Type    string `json:"type"`
	Command string `json:"command,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Error is the stable JSON error payload.
type Error struct {
	Code       string        `json:"code"`
	Message    string        `json:"message"`
	Details    []string      `json:"details,omitempty"`
	Gate       *gate.Failure `json:"gate,omitempty"`
	NextAction *NextAction   `json:"next_action,omitempty"`
	ExitCode   int           `json:"exit_code"`
}

// Envelope is the stable JSON wrapper emitted by automation-facing commands.
type Envelope[T any] struct {
	OK       bool     `json:"ok"`
	Command  string   `json:"command"`
	Warnings []string `json:"warnings,omitempty"`
	Result   T        `json:"result,omitempty"`
	Error    *Error   `json:"error,omitempty"`
}
