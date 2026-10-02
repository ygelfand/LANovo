package clock

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

// setClock steps CLOCK_REALTIME. Stepped rather than slewed: the clock starts decades out, which
// is more than adjtime can carry.
func setClock(t time.Time) error {
	ts := unix.NsecToTimespec(t.UnixNano())
	if err := unix.ClockSettime(unix.CLOCK_REALTIME, &ts); err != nil {
		return fmt.Errorf("clock: setting the clock: %w", err)
	}
	return nil
}
