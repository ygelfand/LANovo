package firmware

import (
	"context"
	"time"
)

// esphome entities do not poll, so a device left alone has to look for releases itself.
const (
	checkEvery  = 24 * time.Hour
	checkSettle = 5 * time.Minute
)

func (f *Firmware) Run(ctx context.Context) error {
	first := time.NewTimer(checkSettle)
	defer first.Stop()

	select {
	case <-ctx.Done():
		return nil
	case <-first.C:
		f.Check(ctx)
	}

	t := time.NewTicker(checkEvery)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			f.Check(ctx)
		}
	}
}
