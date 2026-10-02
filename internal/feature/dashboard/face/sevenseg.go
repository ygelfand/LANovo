package face

import (
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// A seven segment digit, kept apart from the face that arranges them: which bars a digit lights is
// a fact about seven segment displays, not a decision this device makes.

// lit says which of the seven segments each digit turns on, in the order a b c d e f g: top,
// top right, bottom right, bottom, bottom left, top left, middle.
var lit = map[rune][7]bool{
	'0': {true, true, true, true, true, true, false},
	'1': {false, true, true, false, false, false, false},
	'2': {true, true, false, true, true, false, true},
	'3': {true, true, true, true, false, false, true},
	'4': {false, true, true, false, false, true, true},
	'5': {true, false, true, true, false, true, true},
	'6': {true, false, true, true, true, true, true},
	'7': {true, true, true, false, false, false, false},
	'8': {true, true, true, true, true, true, true},
	'9': {true, true, true, true, false, true, true},
}

// drawDigit paints all seven segments, the off ones included: the ghost of a bar that is not lit is
// what makes this a display with lamps in it rather than a typeface.
func Lit(c rune) [7]bool { return lit[c] }

func drawDigit(s ui.Surface, at ui.Rect, c rune, on, off theme.Color) {
	shown := lit[c]

	for i, bar := range Bars(at) {
		color := off
		if shown[i] {
			color = on
		}
		ui.FillRounded(s, bar, min(bar.W, bar.H)/2, color)
	}
}

// bars is where the seven segments of one digit go, in the order lit lists them.
func Bars(at ui.Rect) [7]ui.Rect {
	thick := max(int(float64(at.H)*barThick), 2)
	notch := max(int(float64(at.H)*barNotch), 1)

	// Each bar is inset from the corners by the notch, so two meeting at a corner leave a seam
	// rather than merging into an L.
	long := at.W - thick - notch*2
	tall := (at.H-thick)/2 - notch*2

	mid := at.Y + (at.H-thick)/2

	return [7]ui.Rect{
		{X: at.X + thick/2 + notch, Y: at.Y, W: long, H: thick},
		{X: at.X + at.W - thick, Y: at.Y + thick/2 + notch, W: thick, H: tall},
		{X: at.X + at.W - thick, Y: mid + thick/2 + notch, W: thick, H: tall},
		{X: at.X + thick/2 + notch, Y: at.Y + at.H - thick, W: long, H: thick},
		{X: at.X, Y: mid + thick/2 + notch, W: thick, H: tall},
		{X: at.X, Y: at.Y + thick/2 + notch, W: thick, H: tall},
		{X: at.X + thick/2 + notch, Y: mid, W: long, H: thick},
	}
}

// drawColon is the two dots, which are always lit: a colon that could be off would be read as a
// segment rather than as punctuation.
func drawColon(s ui.Surface, x, top, height int, on theme.Color) {
	dot := max(int(float64(height)*barThick), 2)
	span := int(float64(height) * colonGap)

	for _, y := range []int{top + height/3 - dot/2, top + height*2/3 - dot/2} {
		ui.FillRounded(s, ui.Rect{X: x + (span-dot)/2, Y: y, W: dot, H: dot}, dot/2, on)
	}
}
