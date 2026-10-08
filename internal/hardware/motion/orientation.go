package motion

import (
	"math"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"

	"github.com/ygelfand/LANovo/internal/hardware/display"
)

const (
	still = 0.3

	flat = 0.8

	commit = 0.2
)

type Tracker struct {
	rot   geometry.Orientation
	known bool
}

func NewTracker() *Tracker { return &Tracker{rot: display.Mounted()} }

func (t *Tracker) Orientation() geometry.Orientation { return t.rot }

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

func rotationFor(r Reading) geometry.Orientation {
	best := display.Mounted()
	var strongest float64

	for _, rot := range []geometry.Orientation{
		geometry.Rotate0, geometry.Rotate90, geometry.Rotate180, geometry.Rotate270,
	} {
		if g := gravity(r, rot); g > strongest {
			best, strongest = rot, g
		}
	}
	return best
}

func gravity(r Reading, rot geometry.Orientation) float64 {
	switch rot {
	case geometry.Rotate0:
		return -r.Y
	case geometry.Rotate90:
		return r.X
	case geometry.Rotate180:
		return r.Y
	case geometry.Rotate270:
		return -r.X
	}
	return 0
}

func lead(r Reading, want, have geometry.Orientation) float64 {
	return gravity(r, want) - gravity(r, have)
}
