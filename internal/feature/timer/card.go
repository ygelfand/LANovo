package timer

import (
	"fmt"
	"time"
)

// Card is a timer as the panel shows it.
type Card struct {
	Showing bool
	Ringing bool

	Name string

	// Left is what remains and Of is what it started as, which is what the bar underneath is a
	// fraction of.
	Left time.Duration
	Of   time.Duration
}

// key is what the card says, so the panel is only repainted when that changes rather than on every
// tick of the countdown.
func (c Card) key() string {
	if !c.Showing {
		return ""
	}
	return fmt.Sprintf("%v|%s|%s", c.Ringing, c.Name, Remaining(c.Left))
}

// Remaining is the countdown as it reads: minutes and seconds, and hours only once there are any.
// Rounded up, so a timer started at five minutes says 5:00 rather than 4:59.
func Remaining(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	whole := int((d + time.Second - 1) / time.Second)

	h, m, s := whole/3600, whole/60%60, whole%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
