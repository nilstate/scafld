package contracts

import "testing"

type fakeSubject struct{}

func (fakeSubject) Name() string { return "fake" }

func (fakeSubject) RunContract(t *testing.T) { t.Helper() }

func TestRun(t *testing.T) {
	Run(t, fakeSubject{})
}
