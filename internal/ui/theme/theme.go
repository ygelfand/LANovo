// Package theme is the palette everything on screen draws with.
//
// A theme is only color. What is drawn, and where, belongs to whatever is drawing it — so a new
// screen costs nothing to theme and a new theme costs nothing to apply.
package theme

import (
	"fmt"
	"math"
)

// Color is one color, as the panel takes it.
type Color struct{ R, G, B byte }

// rgb builds a color from the form they are written in, 0xRRGGBB.
func rgb(v uint32) Color {
	return Color{R: byte(v >> 16), G: byte(v >> 8), B: byte(v)}
}

func (c Color) String() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// Blend mixes towards another color, which is how a surface is lifted off its background or a
// label softened without adding another entry to every theme.
func (c Color) Blend(other Color, amount float64) Color {
	switch {
	case amount <= 0:
		return c
	case amount >= 1:
		return other
	}

	mix := func(a, b byte) byte { return byte(float64(a) + (float64(b)-float64(a))*amount) }
	return Color{mix(c.R, other.R), mix(c.G, other.G), mix(c.B, other.B)}
}

// Mix is Blend where the amount is coverage out of 255, which is what a rasterized glyph hands
// over. One call per covered pixel is the hottest arithmetic on the panel, and in fixed point it is
// twice the speed of the same mix in float64 — measured on the device at 45ns a pixel against 96.
//
// Rounded, and over 255 rather than 256, so full coverage lands exactly on the other color rather
// than one short of it. It parts from Blend by at most one step of 255, which no eye resolves and
// a test holds it to.
func (c Color) Mix(other Color, cover byte) Color {
	switch cover {
	case 0:
		return c
	case 0xff:
		return other
	}

	mix := func(a, b byte) byte {
		return byte((int(a)*(255-int(cover)) + int(b)*int(cover) + 127) / 255)
	}
	return Color{mix(c.R, other.R), mix(c.G, other.G), mix(c.B, other.B)}
}

// Theme is a named palette.
//
// The roles are what a screen asks for rather than what a color is: a clock asks for Text and an
// alarm asks for Danger, and neither has to know which theme is on.
type Theme struct {
	Name string
	Dark bool

	// Background is the whole panel; Surface is a panel raised off it, such as a card or the
	// volume overlay.
	Background Color
	Surface    Color

	// Text is what is read; Muted is secondary, such as a date under a time.
	Text  Color
	Muted Color

	// Accent is what draws the eye, Accent2 supports it.
	Accent  Color
	Accent2 Color

	// The three states worth coloring apart from everything else.
	Success Color
	Warning Color
	Danger  Color
}

// ink and paper are the two colors put on an accent. Not pure black and white: both are harsh
// against a saturated fill on a panel this bright.
var (
	ink   = rgb(0x0b0d10)
	paper = rgb(0xf5f7fa)
)

// Contrast is a color that reads against this one, for text or an icon placed on an accent.
// Whichever of the two reads better wins, measured rather than guessed from a threshold.
func (t Theme) Contrast(on Color) Color {
	if Contrast(ink, on) >= Contrast(paper, on) {
		return ink
	}
	return paper
}

// Contrast is the ratio between two colors, 1 for identical and 21 for black against white. It
// is what says whether something can be read.
func Contrast(a, b Color) float64 {
	la, lb := relative(a), relative(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// relative is how bright a color is to an eye, with each channel linearized first.
func relative(c Color) float64 {
	channel := func(v byte) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

// luminance is how bright a color looks, for deciding which way round a theme is.
func luminance(c Color) float64 {
	return (0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)) / 255
}

// Dark reports whether light ink reads better on a color than dark ink does.
//
// Asked of the color rather than read off a theme's Dark flag, because what is being drawn on is
// not always a theme's background: a card is a shade off it, and a photograph behind the clock is
// not a theme's anything. A flag would also be a second place for the same fact to live, and the
// one that can be set wrong.
//
// Measured the same way everything else here is, so a color half way between does not land on one
// answer for the text and the other for the mark beside it.
func Dark(c Color) bool { return Contrast(paper, c) > Contrast(ink, c) }

func (t Theme) On(dark bool) Theme {
	if t.Dark == dark {
		return t
	}
	t.Dark = dark
	if dark {
		t.Background, t.Text = ink, paper
	} else {
		t.Background, t.Text = paper, ink
	}
	t.Surface = t.Background.Blend(t.Text, 0.08)
	t.Muted = t.Text.Blend(t.Background, 0.4)
	return t
}
