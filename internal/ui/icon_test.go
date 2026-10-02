package ui

import (
	"testing"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// A triangle covers about half the square it fits in, and stays inside it.
func TestFillPolygonTriangle(t *testing.T) {
	img := NewImage(40, 40, black)

	FillPolygon(img, []Point{{X: 0, Y: 39}, {X: 20, Y: 0}, {X: 39, Y: 39}}, white)

	// Base 39 by height 39, so about 760 of the square's 1600.
	n := drawn(img, 40, 40, black)
	if n < 680 || n > 860 {
		t.Errorf("a triangle in a 40x40 square filled %d pixels, want about 760", n)
	}
	if img.At(0, 0) != black || img.At(39, 0) != black {
		t.Error("the top corners are filled, and a triangle apex up should leave them")
	}
	if img.At(20, 38) != white {
		t.Error("the middle of the base is not filled")
	}
}

// A shape the wrong way round the screen must be clipped, not written past the buffer.
func TestFillPolygonClips(t *testing.T) {
	img := NewImage(20, 20, black)

	FillPolygon(img, []Point{{X: -50, Y: -50}, {X: 70, Y: -50}, {X: 10, Y: 70}}, white)

	if drawn(img, 20, 20, black) == 0 {
		t.Error("a shape covering the surface drew nothing")
	}
}

// Fewer than three points is not an area.
func TestFillPolygonNeedsThreePoints(t *testing.T) {
	img := NewImage(20, 20, black)

	FillPolygon(img, []Point{{X: 0, Y: 0}, {X: 19, Y: 19}}, white)

	if n := drawn(img, 20, 20, black); n != 0 {
		t.Errorf("a line filled %d pixels", n)
	}
}

// counts how many pixels moved away from the ground they were drawn on.
func drawn(img *Image, w, h int, ground theme.Color) int {
	var n int
	for y := range h {
		for x := range w {
			if img.At(x, y) != ground {
				n++
			}
		}
	}
	return n
}

func TestDrawIconPaints(t *testing.T) {
	img := NewImage(64, 64, black)

	DrawIcon(img, icons.AVVolumeUp, Rect{W: 64, H: 64}, white, black)

	if n := drawn(img, 64, 64, black); n == 0 {
		t.Error("the icon drew nothing")
	}
}

// Different icons have to look different, or naming one is pointless.
func TestIconsDiffer(t *testing.T) {
	up := NewImage(64, 64, black)
	DrawIcon(up, icons.AVVolumeUp, Rect{W: 64, H: 64}, white, black)

	off := NewImage(64, 64, black)
	DrawIcon(off, icons.AVVolumeOff, Rect{W: 64, H: 64}, white, black)

	for y := range 64 {
		for x := range 64 {
			if up.At(x, y) != off.At(x, y) {
				return
			}
		}
	}
	t.Error("two different icons drew the same picture")
}

// The icon is painted in the color it is given, not the one baked into its own palette.
func TestDrawIconUsesTheColorItIsGiven(t *testing.T) {
	img := NewImage(64, 64, black)
	want := theme.Color{R: 0x00, G: 0x80, B: 0xf0}

	DrawIcon(img, icons.AVVolumeUp, Rect{W: 64, H: 64}, want, black)

	for y := range 64 {
		for x := range 64 {
			if img.At(x, y) == want {
				return
			}
		}
	}
	t.Error("no pixel of the icon is the color it was asked for")
}

// Edges are mixed against what is behind them, since a surface takes no alpha. Anything the icon
// touches has to sit between the ground and the color rather than overshoot either.
func TestDrawIconBlendsAgainstTheGround(t *testing.T) {
	img := NewImage(64, 64, black)
	DrawIcon(img, icons.AVVolumeUp, Rect{W: 64, H: 64}, white, black)

	for y := range 64 {
		for x := range 64 {
			c := img.At(x, y)
			if c.R != c.G || c.G != c.B {
				t.Fatalf("%d,%d is %v, want a gray between black and white", x, y, c)
			}
		}
	}
}

// Scaled to whatever square it is given, since the same icon is wanted at a drawer tile's size and
// at a volume column's.
func TestDrawIconScales(t *testing.T) {
	for _, size := range []int{16, 48, 200} {
		img := NewImage(size, size, black)
		DrawIcon(img, icons.AVVolumeUp, Rect{W: size, H: size}, white, black)

		if n := drawn(img, size, size, black); n == 0 {
			t.Errorf("at %dpx the icon drew nothing", size)
		}
	}
}

// Fitted to the middle of a rectangle that is not square, rather than stretched to fill it.
func TestDrawIconCentresInAWideRect(t *testing.T) {
	img := NewImage(200, 60, black)

	DrawIcon(img, icons.AVVolumeUp, Rect{W: 200, H: 60}, white, black)

	// Nothing outside the middle square, which is 60 wide starting at 70.
	for y := range 60 {
		for x := range 200 {
			if x >= 70 && x < 130 {
				continue
			}
			if img.At(x, y) != black {
				t.Fatalf("the icon drew at %d,%d, outside the square it was fitted to", x, y)
			}
		}
	}
}

// An icon larger than the surface is clipped rather than writing past it: the panel is a fixed
// buffer and a write outside it is not something that shows up as a wrong pixel.
func TestDrawIconClips(t *testing.T) {
	img := NewImage(32, 32, black)

	DrawIcon(img, icons.AVVolumeUp, Rect{X: -20, Y: -20, W: 120, H: 120}, white, black)
	DrawIcon(img, icons.AVVolumeUp, Rect{X: 20, Y: 20, W: 120, H: 120}, white, black)
}

// Nothing to draw into is not a reason to fail.
func TestDrawIconInNoSpace(t *testing.T) {
	img := NewImage(32, 32, black)

	DrawIcon(img, icons.AVVolumeUp, Rect{W: 0, H: 0}, white, black)
	DrawIcon(img, icons.AVVolumeUp, Rect{W: -5, H: 10}, white, black)

	if n := drawn(img, 32, 32, black); n != 0 {
		t.Errorf("an icon with no room drew %d pixels", n)
	}
}
