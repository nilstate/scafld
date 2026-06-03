package execution

import "time"

// EnvMode controls whether a process inherits the host environment.
type EnvMode string

const (
	// EnvModeInherit preserves the existing inherited environment behavior.
	EnvModeInherit EnvMode = ""
	// EnvModeExact runs with Request.Env exactly.
	EnvModeExact EnvMode = "exact"
)

// Request describes a command execution requested by an application use case.
type Request struct {
	Command                string
	Args                   []string
	Input                  string
	CWD                    string
	Env                    []string
	EnvMode                EnvMode
	Timeout                time.Duration
	IdleTimeout            time.Duration
	TerminateGrace         time.Duration
	MaxCaptureBytes        int
	ProgressInterval       time.Duration
	SuppressProgressStderr bool
	StdoutEventInspector   func(string) string
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
