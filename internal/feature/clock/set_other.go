//go:build !linux

package clock

import (
	"errors"
	"time"
)

func setClock(time.Time) error { return errors.New("clock: only works on the device") }
