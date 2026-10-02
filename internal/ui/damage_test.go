package ui

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type filling struct {
	*Image
	sets  int
	fills []Rect
}

func (f *filling) Set(x, y int, c theme.Color)    { f.sets++; f.Image.Set(x, y, c) }
func (f *filling) FillRect(r Rect, _ theme.Color) { f.fills = append(f.fills, r) }

func TestAFillThroughAClipIsOneFillOfWhatIsInside(t *testing.T) {
	under := &filling{Image: NewImage(100, 80, black)}
	s := Within(under, Rect{X: 10, Y: 20, W: 50, H: 30})

	FillRect(s, Rect{X: 0, Y: 0, W: 40, H: 40}, theme.Color{R: 1})
	if under.sets != 0 {
		t.Errorf("filled a pixel at a time, %d sets", under.sets)
	}
	if len(under.fills) != 1 || under.fills[0] != (Rect{X: 10, Y: 20, W: 30, H: 20}) {
		t.Errorf("fills %v, want the overlap only", under.fills)
	}

	FillRect(s, Rect{X: 70, Y: 0, W: 10, H: 10}, theme.Color{R: 1})
	if len(under.fills) != 1 {
		t.Errorf("a fill outside the clip reached the surface: %v", under.fills)
	}
}

func TestNothingChangedIsAnEmptyRectangle(t *testing.T) {
	a := NewImage(40, 30, black)
	b := NewImage(40, 30, black)

	if got := Changed(a, b); got.W != 0 || got.H != 0 {
		t.Errorf("two identical pictures differ over %+v", got)
	}
}

// The bounding box is inclusive of the pixels at its edges, which is the whole point: a rectangle
// one pixel short leaves the stale strip this is meant to catch.
func TestTheChangedRectangleHoldsTheEdgePixels(t *testing.T) {
	a := NewImage(40, 30, black)
	b := NewImage(40, 30, black)

	b.Set(10, 5, white)
	b.Set(12, 8, white)

	got := Changed(a, b)
	want := Rect{X: 10, Y: 5, W: 3, H: 4}
	if got != want {
		t.Fatalf("changed over %+v, want %+v", got, want)
	}

	for _, at := range [][2]int{{10, 5}, {12, 8}} {
		if !got.Contains(at[0], at[1]) {
			t.Errorf("the changed rectangle leaves out %d,%d", at[0], at[1])
		}
	}
}

func TestOnePixelChanged(t *testing.T) {
	a := NewImage(40, 30, black)
	b := NewImage(40, 30, black)
	b.Set(0, 0, white)

	if got, want := Changed(a, b), (Rect{W: 1, H: 1}); got != want {
		t.Errorf("one pixel changed over %+v, want %+v", got, want)
	}
}

// Different sizes cannot be compared pixel for pixel, and answering with a small rectangle would be
// worse than answering with all of it.
func TestPicturesOfDifferentSizesChangedEverywhere(t *testing.T) {
	got := Changed(NewImage(40, 30, black), NewImage(50, 20, black))
	if got.W != 50 || got.H != 30 {
		t.Errorf("different sizes changed over %+v, want the larger of each edge", got)
	}
}

func TestARectangleHoldsWhatIsInsideIt(t *testing.T) {
	r := Rect{X: 10, Y: 10, W: 20, H: 20}

	for _, in := range []Rect{
		{X: 10, Y: 10, W: 20, H: 20},
		{X: 15, Y: 15, W: 5, H: 5},
		{X: 29, Y: 29, W: 1, H: 1},
	} {
		if !r.Holds(in) {
			t.Errorf("%+v does not hold %+v", r, in)
		}
	}

	for _, out := range []Rect{
		{X: 9, Y: 10, W: 20, H: 20},
		{X: 10, Y: 10, W: 21, H: 20},
		{X: 10, Y: 10, W: 20, H: 21},
		{X: 30, Y: 30, W: 1, H: 1},
	} {
		if r.Holds(out) {
			t.Errorf("%+v claims to hold %+v", r, out)
		}
	}
}

// Nothing changed is held by anything, including a rectangle that is itself empty: there is no
// stale strip when no pixel moved.
func TestAnEmptyChangeIsHeldByAnything(t *testing.T) {
	if !(Rect{}).Holds(Rect{}) {
		t.Error("an empty rectangle does not hold an empty change")
	}
}

// An empty damage rectangle means the whole screen where the driver reads it, but Holds is asked
// about a real one: a caller that declared nothing and changed something has to fail here.
func TestAnEmptyRectangleHoldsNoRealChange(t *testing.T) {
	if (Rect{}).Holds(Rect{X: 1, Y: 1, W: 2, H: 2}) {
		t.Error("an empty rectangle claims to hold a change")
	}
}
