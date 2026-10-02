package sensors

import "time"

const (
	nearEnter = 2
	nearHold  = 10 * time.Second
	nearStuck = time.Hour
	baseDrop  = 0.25
	baseDrift = 0.01
)

type presence struct {
	base   float64
	based  bool
	near   bool
	above  int
	below  time.Time
	nearAt time.Time
}

func (p *presence) update(v, rise float64, now time.Time) (near, changed bool) {
	fall := max(2, rise/2)
	if !p.based {
		p.base, p.based = v, true
	}
	if v < p.base {
		p.base += baseDrop * (v - p.base)
	}

	if !p.near {
		if v >= p.base+rise {
			p.above++
		} else {
			p.above = 0
			p.base += baseDrift * (v - p.base)
		}
		if p.above >= nearEnter {
			p.near, p.nearAt, p.below = true, now, time.Time{}
			return true, true
		}
		return false, false
	}

	if now.Sub(p.nearAt) >= nearStuck {
		p.base, p.near, p.above = v, false, 0
		return false, true
	}
	if v >= p.base+fall {
		p.below = time.Time{}
		return true, false
	}
	if p.below.IsZero() {
		p.below = now
	}
	if now.Sub(p.below) < nearHold {
		return true, false
	}
	p.near, p.above = false, 0
	return false, true
}
