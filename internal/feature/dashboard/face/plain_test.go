package face

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/ui"
)

// box is a whole screen, which is what the dashboard hands out until it hosts anything else.
func box(w, h int) ui.Rect { return ui.Rect{W: w, H: h} }

func TestArrangeStaysOnScreen(t *testing.T) {
	for _, s := range screens {
		for _, twentyFour := range []bool{true, false} {
			r := Read(at(12, 34), twentyFour)
			l, _ := arrange(r, box(s.w, s.h))

			for _, part := range []struct {
				name string
				r    ui.Rect
			}{{"time", l.Time}, {"suffix", l.Suffix}, {"date", l.Date}} {
				if part.r.W == 0 {
					continue
				}
				if part.r.X < 0 || part.r.Y < 0 {
					t.Errorf("%s: %s starts at %d,%d", s.name, part.name, part.r.X, part.r.Y)
				}
				if part.r.X+part.r.W > s.w {
					t.Errorf("%s: %s runs to %d, past the %d wide screen",
						s.name, part.name, part.r.X+part.r.W, s.w)
				}
				if part.r.Y+part.r.H > s.h {
					t.Errorf("%s: %s runs to %d, past the %d tall screen",
						s.name, part.name, part.r.Y+part.r.H, s.h)
				}
			}
		}
	}
}

// The date belongs under the time, not over it.
func TestArrangePutsTheDateUnderTheTime(t *testing.T) {
	l, _ := arrange(Read(at(12, 34), true), box(1920, 1200))

	if l.Date.Y < l.Time.Y+l.Time.H {
		t.Errorf("the date starts at %d, before the time ends at %d", l.Date.Y, l.Time.Y+l.Time.H)
	}
}

// The suffix sits beside the time, and must not overlap it.
func TestArrangePutsTheSuffixBesideTheTime(t *testing.T) {
	l, _ := arrange(Read(at(15, 30), false), box(1920, 1200))

	if l.Suffix.W == 0 {
		t.Fatal("a 12 hour reading has no suffix placed")
	}
	if l.Suffix.X < l.Time.X+l.Time.W {
		t.Errorf("the suffix starts at %d, inside the time ending at %d",
			l.Suffix.X, l.Time.X+l.Time.W)
	}
}

// The block is centered as a whole, so the same margin is left above and below it.
func TestArrangeCentresTheBlock(t *testing.T) {
	l, _ := arrange(Read(at(12, 34), true), box(1920, 1200))

	above := l.Time.Y
	below := 1200 - (l.Date.Y + l.Date.H)

	if diff := above - below; diff > 4 || diff < -4 {
		t.Errorf("%d above the block and %d below it", above, below)
	}
}

// The box is what it centers in, not the screen. Offsetting the box has to move the face with it,
// or a face handed a corner will draw in the middle of the panel.
func TestArrangeFollowsTheBoxItIsGiven(t *testing.T) {
	r := Read(at(12, 34), true)

	whole, _ := arrange(r, ui.Rect{W: 1200, H: 1920})
	half, _ := arrange(r, ui.Rect{X: 0, Y: 960, W: 1200, H: 960})

	if half.Time.Y <= whole.Time.Y {
		t.Errorf("the lower half put the time at %d, no lower than the whole screen's %d",
			half.Time.Y, whole.Time.Y)
	}
	if half.Time.Y < 960 {
		t.Errorf("the time is at %d, above the box starting at 960", half.Time.Y)
	}
}
