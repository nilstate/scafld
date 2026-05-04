package contracts

import "testing"

type Subject interface {
	Name() string
	RunContract(t *testing.T)
}

func Run(t *testing.T, subjects ...Subject) {
	t.Helper()
	for _, subject := range subjects {
		t.Run(subject.Name(), func(t *testing.T) {
			subject.RunContract(t)
		})
	}
}
