package config

import "github.com/ygelfand/LANovo/internal/lib/say"

import "github.com/ygelfand/LANovo/internal/ui/theme"

// Ink is the color the clock is drawn in.
//
// Kept apart from the theme rather than folded into it. A theme is the whole device — the drawer,
// the settings rows, the volume card — and somebody who wants an amber clock on a dark screen is
// not asking for an amber device. The theme stays what it is and the clock is recolored over it.
type Ink string

const (
	// InkTheme leaves the clock in whatever the theme reads as text, which is what every other
	// screen uses. The default, so a device nobody has set looks like the rest of itself.
	InkTheme Ink = "theme"

	InkAmber  Ink = "amber"
	InkRed    Ink = "red"
	InkGreen  Ink = "green"
	InkCyan   Ink = "cyan"
	InkBlue   Ink = "blue"
	InkViolet Ink = "violet"
	InkPink   Ink = "pink"
)

const DefaultInk = InkTheme

// inks is what each choice draws in.
//
// Mid-luminance and mid-saturation on purpose: the clock is the one thing on the panel that has to
// read against twelve themes, half of them light and half dark, and a color picked to look good on
// one of those is unreadable on the other. Nothing here is a pure hue for the same reason.
//
// InkTheme is absent rather than mapped, because it is not a color: it is the instruction to leave
// the theme alone, and a lookup that misses is exactly that.
var inks = map[Ink]theme.Color{
	InkAmber:  {R: 0xf2, G: 0xa6, B: 0x3b},
	InkRed:    {R: 0xe5, G: 0x54, B: 0x4b},
	InkGreen:  {R: 0x4c, G: 0xaf, B: 0x78},
	InkCyan:   {R: 0x35, G: 0xb8, B: 0xc4},
	InkBlue:   {R: 0x4f, G: 0x8e, B: 0xf7},
	InkViolet: {R: 0x9a, G: 0x7b, B: 0xf0},
	InkPink:   {R: 0xe8, G: 0x6a, B: 0xa6},
}

// mutedOfInk is how far the date under the time is mixed back toward the background.
//
// The clock is one color, not two: a colored time over a gray date reads as the date belonging to
// something else. Softened by the same fraction the themes soften their own text by, so the pair
// sits apart the way it does on every other screen.
const mutedOfInk = 0.45

// inkFloor is the contrast a clock has to reach against the background it is drawn on.
//
// Four and a half to one, which is the usual floor for body text and more than large numerals
// strictly need. Deliberately over-specified: this is a wall clock read across a room and at an
// angle, which is worse than the seated reading those floors assume.
const inkFloor = 4.5

// readable is a color turned into one that can be read on a theme.
//
// The same amber cannot serve twelve themes. At full strength it is right on the dark ones and
// close to invisible on Paper, where it reads 1.87 to 1 — the panel is bright and a light theme is
// nearly white. So the hue is kept and the shade is given up: the color is mixed toward whatever
// the theme already reads as text, a little at a time, until it clears the floor.
//
// Toward the theme's own text rather than toward black or white, because that color is readable on
// this background by construction — it is what every other screen is drawn in — so the walk always
// terminates somewhere legible, and on a dark theme it terminates immediately having changed
// nothing. What comes back is as much of the color as the theme can carry.
func readable(c theme.Color, t theme.Theme) theme.Color {
	const step = 0.05

	for by := 0.0; by < 1; by += step {
		got := c.Blend(t.Text, by)
		if theme.Contrast(got, t.Background) >= inkFloor {
			return got
		}
	}
	return t.Text
}

func (i Ink) Label() string {
	switch i {
	case InkTheme:
		return say.T("ink.theme")
	case InkAmber:
		return say.T("ink.amber")
	case InkRed:
		return say.T("ink.red")
	case InkGreen:
		return say.T("ink.green")
	case InkCyan:
		return say.T("ink.cyan")
	case InkBlue:
		return say.T("ink.blue")
	case InkViolet:
		return say.T("ink.violet")
	case InkPink:
		return say.T("ink.pink")
	}
	return string(i)
}

// Color is what the clock is drawn in over a theme, which is the theme's own text unless something
// else was chosen. For a swatch, and for anything that wants the color without the whole palette.
func (i Ink) Color(t theme.Theme) theme.Color {
	if c, ok := inks[i]; ok {
		return readable(c, t)
	}
	return t.Text
}

// Over is the theme a face is handed: the same one everything else draws with, with the two roles
// the clock reads as recolored.
//
// Text and Muted only. Accent is left alone deliberately — it is the second hand, and a second hand
// in the same color as the hour and minute hands is a hand nobody can pick out. Surface is left
// alone because it is the card behind the cards face and the unlit lamps behind the segments, both
// of which are the object the clock is drawn on rather than the clock.
func (i Ink) Over(t theme.Theme) theme.Theme {
	if _, ok := inks[i]; !ok {
		return t
	}

	c := i.Color(t)

	t.Text = c
	t.Muted = c.Blend(t.Background, mutedOfInk)
	return t
}

// Inks is every color the clock can be drawn in, with Theme first: it is the one somebody comes
// back to, rather than one more color in the row.
func Inks() []Ink {
	return []Ink{InkTheme, InkAmber, InkRed, InkGreen, InkCyan, InkBlue, InkViolet, InkPink}
}
