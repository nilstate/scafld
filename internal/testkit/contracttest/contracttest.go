package contracttest

import "testing"

// Subject is an implementation that can run a shared conformance test suite.
type Subject interface {
	Name() string
	RunContract(t *testing.T)
}

// Run executes each subject's conformance test suite.
func Run(t *testing.T, subjects ...Subject) {
	t.Helper()
	for _, subject := range subjects {
		t.Run(subject.Name(), func(t *testing.T) {
			subject.RunContract(t)
		})
	}
}
