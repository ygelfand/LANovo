package clock

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

func setClock(t time.Time) error {
	ts := unix.NsecToTimespec(t.UnixNano())
	if err := unix.ClockSettime(unix.CLOCK_REALTIME, &ts); err != nil {
		return fmt.Errorf("clock: setting the clock: %w", err)
	}
	return nil
}
