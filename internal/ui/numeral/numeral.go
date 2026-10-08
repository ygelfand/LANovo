package numeral

import (
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type Set struct {
	Name string

	BoxW, BoxH int

	Digits [10]ui.Icon
	Colon  ui.Icon
}

func (s Set) Wide(tall int) int {
	if s.BoxH <= 0 {
		return 0
	}
	return tall * s.BoxW / s.BoxH
}

func (s Set) Tall(wide int) int {
	if s.BoxW <= 0 {
		return 0
	}
	return wide * s.BoxH / s.BoxW
}

func (s Set) Digit(dst ui.Surface, d int, at ui.Rect, fg, on theme.Color) {
	if d < 0 || d > 9 {
		return
	}
	ui.DrawIconFilling(dst, s.Digits[d], at, fg, on)
}

const colonCell = 4

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

		x := at.X + at.W*done/total
		to := at.X + at.W*(done+u)/total
		done += u

		switch {
		case r >= '0' && r <= '9':
			s.Digit(dst, int(r-'0'), ui.Rect{X: x, Y: at.Y, W: to - x, H: at.H}, fg, on)

		case r == ':':
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

func (s Set) Measure(text string, tall int) (w, h int) {
	return s.Wide(tall) * units([]rune(text)) / ninths, tall
}

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
