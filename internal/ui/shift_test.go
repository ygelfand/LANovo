package ui

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

var (
	back = theme.Color{R: 0, G: 0, B: 0}
	ink  = theme.Color{R: 255, G: 255, B: 255}
)

func TestShiftMovesWhatIsDrawn(t *testing.T) {
	img := NewImage(40, 20, back)
	s := Shift(img, 10, 5)

	s.Set(0, 0, ink)

	if got := img.At(10, 5); got != ink {
		t.Errorf("the pixel drawn at the origin is at 10,5 as %v, want %v", got, ink)
	}
	if got := img.At(0, 0); got != back {
		t.Errorf("something was drawn at the origin: %v", got)
	}
}

// A page laid out for the whole screen and then moved has to still be told the whole screen, or it
// would lay itself out for the sliver that is showing.
func TestShiftKeepsTheSizeOfWhatIsUnderIt(t *testing.T) {
	img := NewImage(40, 20, back)

	w, h := Shift(img, 10, 5).Size()
	if w != 40 || h != 20 {
		t.Errorf("a shifted surface is %dx%d, want 40x20", w, h)
	}
}

func TestShiftFillsAtTheOffset(t *testing.T) {
	img := NewImage(40, 20, back)

	FillRect(Shift(img, 10, 0), Rect{W: 5, H: 20}, ink)

	if got := img.At(12, 10); got != ink {
		t.Errorf("the band is not at 12: %v", got)
	}
	if got := img.At(2, 10); got != back {
		t.Errorf("the band was painted at 2, before the offset: %v", got)
	}
}

// Everything past the edge belongs to the page on the other side of the slide, so it has to be
// dropped rather than wrapped round.
func TestShiftPastTheEdgeDrawsNothing(t *testing.T) {
	img := NewImage(40, 20, back)

	FillRect(Shift(img, 38, 0), Rect{W: 40, H: 20}, ink)

	if got := img.At(0, 0); got != back {
		t.Errorf("paint pushed off the right came back on the left: %v", got)
	}
	if got := img.At(39, 0); got != ink {
		t.Errorf("the part still on the surface was not painted: %v", got)
	}
}

// Drawing asks what it is allowed to paint so it can skip the rest, and the answer has to be in
// the coordinates the caller is using rather than the ones underneath.
func TestShiftMovesTheClipBack(t *testing.T) {
	img := NewImage(40, 20, back)
	clipped := clip{Surface: img, at: Rect{X: 10, Y: 5, W: 20, H: 10}}

	got := ClipOf(Shift(clipped, 10, 5))
	want := Rect{X: 0, Y: 0, W: 20, H: 10}
	if got != want {
		t.Errorf("the clip is %+v, want %+v", got, want)
	}
}

// Most of a page part way through a slide is past the edge of the screen. Saying so is what keeps
// it from rasterizing text nobody will see, which is the whole cost of drawing two pages at once.
func TestShiftOfAnUnclippedSurfaceIsBoundedByItsEdges(t *testing.T) {
	img := NewImage(40, 20, back)

	got := ClipOf(Shift(img, -30, 0))
	want := Rect{X: 30, Y: 0, W: 40, H: 20}
	if got != want {
		t.Errorf("the clip is %+v, want %+v", got, want)
	}
}

// clip is a surface that is only accepting part of itself, which the panel is mid-frame and an
// image never is.
type clip struct {
	Surface
	at Rect
}

func (c clip) Clipped() Rect { return c.at }
