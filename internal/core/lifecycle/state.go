package lifecycle

// State names a lifecycle state in the task workflow.
type State string

const (
	// Draft is the editable pre-approval state.
	Draft State = "draft"
	// Approved is the state after approval but before execution.
	Approved State = "approved"
	// Active is the execution state.
	Active State = "active"
	// Blocked is the state after execution or review finds blocking work.
	Blocked State = "blocked"
	// Review is the adversarial review gate state.
	Review State = "review"
	// Completed is the terminal successful state.
	Completed State = "completed"
	// Failed is the terminal unsuccessful state.
	Failed State = "failed"
	// Cancelled is the terminal abandoned state.
	Cancelled State = "cancelled"
)

// CanTransition reports whether the lifecycle transition is allowed.
func CanTransition(from State, to State) bool {
	if from == to {
		return true
	}
	for _, allowed := range transitions[from] {
		if to == allowed {
			return true
		}
	}
	return false
}

var transitions = map[State][]State{
	Draft:    {Approved, Cancelled},
	Approved: {Active, Cancelled},
	Active:   {Blocked, Review, Failed, Cancelled},
	Blocked:  {Active, Failed, Cancelled},
	Review:   {Active, Completed, Failed, Cancelled},
}
