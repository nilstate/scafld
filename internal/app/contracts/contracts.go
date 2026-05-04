package contracts

type NextAction struct {
	Type    string `json:"type"`
	Command string `json:"command,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type Error struct {
	Code       string      `json:"code"`
	Message    string      `json:"message"`
	Details    []string    `json:"details,omitempty"`
	NextAction *NextAction `json:"next_action,omitempty"`
	ExitCode   int         `json:"exit_code"`
}

type Envelope[T any] struct {
	OK       bool     `json:"ok"`
	Command  string   `json:"command"`
	Warnings []string `json:"warnings,omitempty"`
	Result   T        `json:"result,omitempty"`
	Error    *Error   `json:"error,omitempty"`
}
