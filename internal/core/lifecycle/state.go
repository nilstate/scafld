package lifecycle

type State string

const (
	Draft     State = "draft"
	Approved  State = "approved"
	Active    State = "active"
	Blocked   State = "blocked"
	Review    State = "review"
	Completed State = "completed"
	Failed    State = "failed"
	Cancelled State = "cancelled"
)

func CanTransition(from State, to State) bool {
	if from == to {
		return true
	}
	switch from {
	case Draft:
		return to == Approved || to == Cancelled
	case Approved:
		return to == Active || to == Cancelled
	case Active:
		return to == Blocked || to == Review || to == Failed || to == Cancelled
	case Blocked:
		return to == Active || to == Failed || to == Cancelled
	case Review:
		return to == Active || to == Completed || to == Failed || to == Cancelled
	default:
		return false
	}
}
