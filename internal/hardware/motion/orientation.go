package motion

import (
	"math"

	"github.com/ygelfand/LANovo/internal/hardware/display"
)

const (
	still = 0.3

	flat = 0.8

	commit = 0.2
)

type Tracker struct {
	rot   display.Orientation
	known bool
}

func NewTracker() *Tracker { return &Tracker{rot: display.Mounted()} }

func (t *Tracker) Orientation() display.Orientation { return t.rot }

func (t *Tracker) Update(r Reading) (changed bool) {
	if math.Abs(r.Magnitude()-1) > still {
		return false
	}
	if math.Abs(r.Z) > flat {
		return false
	}

	want := rotationFor(r)

	if t.known && want != t.rot && lead(r, want, t.rot) < commit {
		return false
	}

	t.known = true
	if want == t.rot {
		return false
	}
	t.rot = want
	return true
}

func rotationFor(r Reading) display.Orientation {
	best := display.Mounted()
	var strongest float64

	for _, rot := range []display.Orientation{
		display.Rotate0, display.Rotate90, display.Rotate180, display.Rotate270,
	} {
		if g := gravity(r, rot); g > strongest {
			best, strongest = rot, g
		}
	}
	return best
}

func gravity(r Reading, rot display.Orientation) float64 {
	switch rot {
	case display.Rotate0:
		return -r.Y
	case display.Rotate90:
		return r.X
	case display.Rotate180:
		return r.Y
	case display.Rotate270:
		return -r.X
	}
	return 0
}

func lead(r Reading, want, have display.Orientation) float64 {
	return gravity(r, want) - gravity(r, have)
}
