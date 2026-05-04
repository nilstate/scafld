package clock

import "time"

// System reads time from the host system clock.
type System struct{}

// Now returns the current UTC time.
func (System) Now() time.Time {
	return time.Now().UTC()
}
