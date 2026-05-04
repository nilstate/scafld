package execution

import "time"

// Request describes a command execution requested by an application use case.
type Request struct {
	Command              string
	Args                 []string
	Input                string
	CWD                  string
	Env                  []string
	Timeout              time.Duration
	IdleTimeout          time.Duration
	TerminateGrace       time.Duration
	MaxCaptureBytes      int
	StdoutEventInspector func(string) string
}

// Result captures the observable outcome of a command execution.
type Result struct {
	ExitCode          int
	Output            string
	Stdout            string
	Stderr            string
	DiagnosticPath    string
	TimedOut          bool
	KillReason        string
	WallElapsed       time.Duration
	TimeSinceLastByte time.Duration
	IdleTimeout       time.Duration
	AbsoluteTimeout   time.Duration
	DroppedBytes      int
	StdoutEvents      map[string]int
}
