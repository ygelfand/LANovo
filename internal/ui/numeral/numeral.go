// Package numeral is digits drawn as paths, so a clock face can have its own.
//
// Most of what a clock picker calls a style is a typeface. Embedding display fonts for that is the
// obvious answer and the wrong one: a clock needs ten shapes and a colon, and a font is a few
// hundred kilobytes of machinery for setting words. A digit is a filled path, and the panel already
// rasterizes filled paths — so a set is eleven blobs of a few hundred bytes, through a pipeline that
// exists.
//
// Tabular by construction. Every digit is drawn into the same box and laid out by that box, so the
// time cannot shift as the digits change. Proportional figures move the whole clock every minute;
// here that is not something to remember, it is unrepresentable.
package numeral

import (
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Set is one way of drawing the digits.
//
// The digits are an array indexed by the digit, so drawing one is an index rather than a switch,
// and a set is one name rather than eleven.
type Set struct {
	Name string

	// BoxW and BoxH are the box every glyph was drawn in, from the SVG's viewBox. Carried so a face
	// can lay digits out by arithmetic and never by measuring a mask it would have to rasterize
	// first.
	BoxW, BoxH int

	Digits [10]ui.Icon
	Colon  ui.Icon
}

// Wide is how wide a digit is when drawn tall pixels high, which is the only question a face laying
// out a row of them has.
func (s Set) Wide(tall int) int {
	if s.BoxH <= 0 {
		return 0
	}
	return tall * s.BoxW / s.BoxH
}

// Tall is the other way round: the height that gives a digit of this width. For a face bounded by
// the width of its box rather than the height.
func (s Set) Tall(wide int) int {
	if s.BoxW <= 0 {
		return 0
	}
	return wide * s.BoxH / s.BoxW
}

// Digit paints one digit filling a box. Out of range draws nothing rather than panicking: the
// digits come from formatting a time, and a clock is not worth taking the screen down for.
func (s Set) Digit(dst ui.Surface, d int, at ui.Rect, fg, on theme.Color) {
	if d < 0 || d > 9 {
		return
	}
	ui.DrawIconFilling(dst, s.Digits[d], at, fg, on)
}

// colonCell is how much of a digit's room the colon is given, in ninths.
//
// Less than a digit, because a colon is two dots and a full cell leaves a hole in the middle of the
// clock. Still tabular: a colon is always a colon, so the run is the same width every minute, which
// is the property that matters. Only a glyph whose width changed with what it said would shift the
// clock, and none here does.
const colonCell = 4

// Draw paints a run of digits and colons, left to right, each in its own cell.
//
// The cells are fixed, so the clock does not move as the digits change. What a face has to do is
// hand over a box; what it must never have to do is measure a mask, which would mean rasterizing
// one first.
func (s Set) Draw(dst ui.Surface, text string, at ui.Rect, fg, on theme.Color) {
	runes := []rune(text)
	if len(runes) == 0 || at.W <= 0 || at.H <= 0 {
		return
	}

	total := units(runes)
	if total == 0 {
		return
	}

	var done int
	for _, r := range runes {
		u := unit(r)

		// Edges measured from the box rather than stepped, so rounding cannot leave the last glyph
		// a pixel short of the right edge.
		x := at.X + at.W*done/total
		to := at.X + at.W*(done+u)/total
		done += u

		switch {
		case r >= '0' && r <= '9':
			s.Digit(dst, int(r-'0'), ui.Rect{X: x, Y: at.Y, W: to - x, H: at.H}, fg, on)

		case r == ':':
			// Drawn at a digit's width, centered on its narrower cell. The colon's ink is a sliver
			// in the middle of its design box, so the overhang is empty and squeezing the box
			// instead would squash the dots out of round.
			//
			// Slid back inside the run rather than left to overhang it. A leading or trailing colon
			// has nothing beside it to overhang into, and at small sizes the ink follows the box
			// out. Sliding keeps the dots round where squeezing would not; only a colon wider than
			// the whole run has to give up and take what there is.
			wide := s.Wide(at.H)
			mid := (x + to) / 2

			box := ui.Rect{X: mid - wide/2, Y: at.Y, W: wide, H: at.H}
			if box.W >= at.W {
				box.X, box.W = at.X, at.W
			} else {
				box.X = max(at.X, min(box.X, at.X+at.W-box.W))
			}

			ui.DrawIconFilling(dst, s.Colon, box, fg, on)
		}
	}
}

// Measure is the box a run of characters needs to be tall pixels high.
func (s Set) Measure(text string, tall int) (w, h int) {
	return s.Wide(tall) * units([]rune(text)) / ninths, tall
}

// ninths is what a digit's cell is worth, so a colon can be a fraction of one without floats.
const ninths = 9

func unit(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return ninths
	case r == ':':
		return colonCell
	}
	return 0
}

func units(runes []rune) int {
	var n int
	for _, r := range runes {
		n += unit(r)
	}
	return n
}
