package projection

import "github.com/nilstate/scafld/v2/internal/core/spec"

type Status struct {
	TaskID string
	Status spec.Status
	Next   string
}

func FromSpec(model spec.Model) Status {
	return Status{TaskID: model.TaskID, Status: model.Status, Next: model.CurrentState.AllowedFollowUp}
}
