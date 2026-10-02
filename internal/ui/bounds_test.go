package ui

import (
	"testing"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Every primitive that is handed a rectangle draws inside it.
//
// This is the contract everything else in the tree is built on: a face, a row, a card and an
// overlay all place things by working out a rectangle and calling one of these. FillRounded was
// painting one pixel outside itself for years and nothing noticed, which is the argument for
// checking the rest rather than assuming.
//
// The shapes are deliberately awkward — one pixel, one dimension far larger than the other, an even
// and an odd size — because that is where rounding and centring go wrong.
func TestEveryPrimitiveStaysInsideTheRectangleItIsGiven(t *testing.T) {
	palette := theme.Default()

	art := NewImage(37, 61, palette.Accent)
	FillRect(art, Rect{W: 20, H: 20}, palette.Danger)

	shapes := []Rect{
		{W: 1, H: 1},
		{W: 2, H: 2},
		{W: 3, H: 40},
		{W: 40, H: 3},
		{W: 41, H: 41},
		{W: 64, H: 64},
		{W: 120, H: 37},
	}

	draws := []struct {
		name string
		draw func(s Surface, r Rect)
	}{
		{"FillRect", func(s Surface, r Rect) { FillRect(s, r, white) }},
		{"FillRounded 1", func(s Surface, r Rect) { FillRounded(s, r, 1, white) }},
		{"FillRounded half", func(s Surface, r Rect) { FillRounded(s, r, min(r.W, r.H)/2, white) }},
		{"FillRounded huge", func(s Surface, r Rect) { FillRounded(s, r, max(r.W, r.H), white) }},
		{"DrawIcon", func(s Surface, r Rect) { DrawIcon(s, icons.ActionSettings, r, white, black) }},
		{"DrawIconFilling", func(s Surface, r Rect) {
			DrawIconFilling(s, icons.ActionSettings, r, white, black)
		}},
		{"DrawLogo", func(s Surface, r Rect) { DrawLogo(s, r, black) }},
		{"DrawScaled", func(s Surface, r Rect) { DrawScaled(s, art, r) }},
		{"FillPolygon", func(s Surface, r Rect) {
			FillPolygon(s, []Point{
				{X: r.X, Y: r.Y},
				{X: r.X + r.W - 1, Y: r.Y},
				{X: r.X + r.W/2, Y: r.Y + r.H - 1},
			}, white)
		}},
	}

	// Room on all four sides, so an overrun in any direction lands where it can be seen.
	const pad = 12

	for _, d := range draws {
		for _, shape := range shapes {
			at := Rect{X: pad, Y: pad, W: shape.W, H: shape.H}
			w, h := at.X+at.W+pad, at.Y+at.H+pad

			blank := NewImage(w, h, black)
			img := NewImage(w, h, black)

			d.draw(img, at)

			if painted := Changed(blank, img); !at.Holds(painted) {
				t.Errorf("%s in %dx%d painted %+v, outside %+v", d.name, shape.W, shape.H, painted, at)
			}
		}
	}
}

// An image drawn at a point covers its own size from there and no more.
func TestDrawingAnImageCoversItsOwnSize(t *testing.T) {
	palette := theme.Default()
	art := NewImage(23, 17, palette.Accent)

	const pad = 10
	img := NewImage(23+2*pad, 17+2*pad, black)
	blank := NewImage(23+2*pad, 17+2*pad, black)

	Draw(img, art, pad, pad)

	at := Rect{X: pad, Y: pad, W: 23, H: 17}
	if painted := Changed(blank, img); !at.Holds(painted) {
		t.Errorf("an image painted %+v, outside %+v", painted, at)
	}
}

// Text centred in a box stays in it, as long as it fits. What happens when it does not is #141.
func TestTextThatFitsStaysInItsBox(t *testing.T) {
	const pad = 20
	font := MustLoad(Regular, 18)

	at := Rect{X: pad, Y: pad, W: 200, H: 40}
	w, h := at.X+at.W+pad, at.Y+at.H+pad

	for _, s := range []string{"Media", "100%", "Wi-Fi", "gyp,j", "|||"} {
		blank := NewImage(w, h, black)
		img := NewImage(w, h, black)

		DrawTextIn(img, font, at, white, black, s)

		if painted := Changed(blank, img); !at.Holds(painted) {
			t.Errorf("%q painted %+v, outside %+v", s, painted, at)
		}
	}
}
