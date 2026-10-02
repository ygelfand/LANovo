package motion

import (
	"math"

	"github.com/ygelfand/LANovo/internal/hardware/display"
)

// The thresholds that keep a rotation from flapping.
const (
	// still is how far from 1g a reading may be and still be gravity rather than a device being
	// picked up or set down.
	still = 0.3

	// flat is how much of gravity has to be on the screen's own axis before the device is lying
	// face up or down, where there is no rotation to read.
	flat = 0.8

	// commit is how much further the new rotation has to lead the old one before it is taken. A
	// device resting near a boundary otherwise turns back and forth.
	commit = 0.2
)

// Tracker turns readings into a rotation, holding the last one until a new one is clearly right.
type Tracker struct {
	rot   display.Orientation
	known bool
}

// NewTracker starts from how the device is usually stood.
func NewTracker() *Tracker { return &Tracker{rot: display.Mounted()} }

// Orientation is the rotation the picture should be drawn at.
func (t *Tracker) Orientation() display.Orientation { return t.rot }

// Update takes one reading and reports whether the rotation changed.
//
// A reading is ignored when the device is being moved, and when it is lying flat: neither says
// anything about which edge is up.
func (t *Tracker) Update(r Reading) (changed bool) {
	if math.Abs(r.Magnitude()-1) > still {
		return false
	}
	if math.Abs(r.Z) > flat {
		return false
	}

	want := rotationFor(r)

	// The first reading settles it; after that the new candidate has to lead by enough to be
	// worth turning the screen for.
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

// rotationFor is the rotation whose down direction best matches gravity.
//
// The signs are the part's axes as this board mounts them, established by standing the device up
// and watching which way the picture came out rather than from the datasheet.
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

// gravity is how much of the reading points along the bottom of the screen at a rotation. The
// largest wins.
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

// lead is how far the candidate is ahead of what is currently shown.
func lead(r Reading, want, have display.Orientation) float64 {
	return gravity(r, want) - gravity(r, have)
}
