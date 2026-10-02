package speaker

import "time"

const (
	driftSettle = 5 * time.Second
	driftSmooth = 10 * time.Second
	driftDrain  = time.Minute
	driftMost   = 500.0
)

type Drift struct {
	began    time.Time
	last     time.Time
	avg      float64
	baseline float64
	settled  bool
}

func (d *Drift) Reset() { *d = Drift{} }

func (d *Drift) Observe(queued int, now time.Time) float64 {
	q := float64(queued)
	if d.began.IsZero() {
		d.began, d.last, d.avg = now, now, q
		return 0
	}

	dt := now.Sub(d.last)
	d.last = now
	d.avg += (q - d.avg) * min(1, float64(dt)/float64(driftSmooth))

	if !d.settled {
		if now.Sub(d.began) < driftSettle {
			return 0
		}
		d.baseline, d.settled = d.avg, true
	}

	ppm := (d.avg - d.baseline) / (Rate * driftDrain.Seconds()) * 1e6
	return max(-driftMost, min(driftMost, ppm))
}
