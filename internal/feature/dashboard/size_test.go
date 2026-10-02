package dashboard

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

// Small is smaller than medium is smaller than large, on both sides. Obvious, and the one thing the
// setting actually promises.
func TestTheSizesAreInOrder(t *testing.T) {
	for _, at := range config.Positions() {
		for _, s := range screens {
			var last config.Size
			for _, size := range config.Sizes() {
				box := Box(at, size, s.w, s.h)

				if last != "" {
					was := Box(at, last, s.w, s.h)
					if box.W <= was.W || box.H <= was.H {
						t.Errorf("%s %s: %s is %dx%d, not bigger than %s at %dx%d",
							at, s.name, size, box.W, box.H, last, was.W, was.H)
					}
				}
				last = size
			}
		}
	}
}

// A smaller clock keeps its shape rather than being squeezed: a face fits itself to the box, so a
// box that shrank on one side only would stretch the face rather than scale it.
func TestShrinkingKeepsTheShape(t *testing.T) {
	const tolerance = 0.01

	for _, at := range config.Positions() {
		for _, s := range screens {
			full := Box(at, config.SizeLarge, s.w, s.h)

			for _, size := range config.Sizes() {
				box := Box(at, size, s.w, s.h)

				wide := float64(box.W) / float64(full.W)
				tall := float64(box.H) / float64(full.H)

				if diff := wide - tall; diff > tolerance || diff < -tolerance {
					t.Errorf("%s %s %s: kept %.3f of the width and %.3f of the height",
						at, size, s.name, wide, tall)
				}
			}
		}
	}
}

// Size must not move the clock off the edge the position names. Scaling a top band about its middle
// would put a small clock a third of the way down the screen, which is not the top anybody asked
// for — so top stays against the top and bottom against the bottom, whatever the size.
func TestSizeDoesNotDragTheClockOffItsEdge(t *testing.T) {
	for _, s := range screens {
		for _, size := range config.Sizes() {
			top := Box(config.PositionTop, size, s.w, s.h)
			if top.Y != 0 {
				t.Errorf("%s %s: the top box starts %d down", size, s.name, top.Y)
			}

			bottom := Box(config.PositionBottom, size, s.w, s.h)
			if got := bottom.Y + bottom.H; got != s.h {
				t.Errorf("%s %s: the bottom box ends at %d, not %d", size, s.name, got, s.h)
			}
		}
	}
}

// Centered stays centered at every size, which is the other half of the same promise.
func TestACenteredClockStaysCentered(t *testing.T) {
	for _, s := range screens {
		for _, size := range config.Sizes() {
			box := Box(config.PositionCenter, size, s.w, s.h)

			// Off by at most one, since halving an odd number of leftover pixels cannot be even.
			if gap := (box.X) - (s.w - box.X - box.W); gap > 1 || gap < -1 {
				t.Errorf("%s %s: %d px to the left, %d to the right",
					size, s.name, box.X, s.w-box.X-box.W)
			}
			if gap := (box.Y) - (s.h - box.Y - box.H); gap > 1 || gap < -1 {
				t.Errorf("%s %s: %d px above, %d below",
					size, s.name, box.Y, s.h-box.Y-box.H)
			}
		}
	}
}

// A size this build does not have fills what it is given rather than collapsing to nothing.
func TestAnUnknownSizeIsLarge(t *testing.T) {
	got := Box(config.PositionCenter, config.Size("enormous"), 1200, 1920)

	if want := Box(config.PositionCenter, config.SizeLarge, 1200, 1920); got != want {
		t.Errorf("an unknown size is %v, want the large %v", got, want)
	}
}
