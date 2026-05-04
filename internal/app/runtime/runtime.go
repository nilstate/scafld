package runtime

import "github.com/nilstate/scafld/v2/internal/core/spec"

func NextAction(model spec.Model) string {
	if model.CurrentState.AllowedFollowUp != "" {
		return model.CurrentState.AllowedFollowUp
	}
	switch model.Status {
	case spec.StatusDraft:
		return "scafld approve " + model.TaskID
	case spec.StatusApproved:
		return "scafld build " + model.TaskID
	case spec.StatusReview:
		return "scafld complete " + model.TaskID
	default:
		return "scafld status " + model.TaskID
	}
}
