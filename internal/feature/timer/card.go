package timer

import (
	"fmt"
	"time"
)

type Card struct {
	Showing bool
	Ringing bool

	Name string

	Left time.Duration
	Of   time.Duration
}

func (c Card) key() string {
	if !c.Showing {
		return ""
	}
	return fmt.Sprintf("%v|%s|%s", c.Ringing, c.Name, Remaining(c.Left))
}

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
