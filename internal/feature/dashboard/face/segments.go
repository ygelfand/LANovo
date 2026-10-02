package face

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() { register(config.FaceSegments, segments{}) }

// segments is a seven segment display: four digits and a colon, with the segments that are off
// drawn faintly.
//
// It draws its own numerals rather than setting type, because a font has no way to say what a glyph
// would have looked like, and the ghost of the unlit bars is the whole of the effect. The digit
// itself is in sevenseg.go; this file is how they are arranged and sized.
type segments struct{}

// The block, against the box.
//
// Two shares because the face turns: four digits and a colon across a portrait panel are bound by
// the width and come out a sixth of the height tall, which is a display lost in the middle of the
// glass. Stacked, each row is two digits and the height is what binds.
const (
	segmentsWidthShare  = 0.88
	segmentsHeightShare = 0.34
	segmentsRowShare    = 0.26
	segmentsRowGap      = 0.22
)

// A digit and its parts, against the digit's own height.
const (
	digitAspect = 0.56
	digitGap    = 0.16
	colonGap    = 0.30

	// The bar, and the notch between bars so they do not run into one another at the corners.
	barThick = 0.15
	barNotch = 0.030

	// How far the off segments are taken from the surface towards the text. Enough to see in a lit
	// room, not enough to read as a digit.
	segmentGhost = 0.09

	segmentsDateOf  = 0.13
	segmentsDateGap = 0.34
)

// segmentsCell is one lamp position: where it is and what is showing in it.
//
// The character is part of the layout rather than something read off the reading again, because
// what makes this face cheap to repaint is knowing which of the four cells changed — and a cell
// keeps its place whatever is in it.
type segmentsCell struct {
	At ui.Rect
	Ch rune
}

// segmentsLayout is where the pieces land in a box.
type segmentsLayout struct {
	Cells []segmentsCell

	// Date is the line under the display, empty when there is no date.
	Date ui.Rect
}

// segmentsArrange places the lamps, so that drawing them and saying where they are cannot disagree.
func segmentsArrange(in ui.Rect, r Reading) (segmentsLayout, *ui.Font) {
	hour, minute, ok := lines(r)
	if !ok {
		return segmentsLayout{}, nil
	}

	// One line across, two lines down. Stacked drops the colon: what separates the hour from the
	// minutes is the gap between the rows, and a colon left in would be a lamp with nothing to say.
	rows := []string{hour + ":" + minute}
	if in.H > in.W {
		rows = []string{hour, minute}
	}

	height := segmentsFit(in, rows)

	width := int(float64(height) * digitAspect)
	gap := int(float64(height) * digitGap)
	between := int(float64(height) * segmentsRowGap)
	colon := int(float64(height) * colonGap)

	date := ui.MustLoad(ui.Regular, max(int(float64(height)*segmentsDateOf), 1))

	dateW, dateH, under := 0, 0, 0
	if r.Dated() {
		dateW, dateH = date.Measure(r.Date)
		under = int(float64(height) * segmentsDateGap)
	}

	tall := height*len(rows) + between*(len(rows)-1)
	top := in.Y + (in.H-(tall+under+dateH))/2

	var l segmentsLayout
	for _, row := range rows {
		x := in.X + (in.W-runWidth(row, width, gap, height))/2

		for _, c := range row {
			cell := width
			if c == ':' {
				cell = colon
			}

			l.Cells = append(l.Cells, segmentsCell{
				At: ui.Rect{X: x, Y: top, W: cell, H: height},
				Ch: c,
			})
			x += cell + gap
		}
		top += height + between
	}

	// top has walked past the last row and one gap too many, which is where the date goes from.
	top -= between

	if r.Dated() {
		l.Date = ui.Rect{X: in.X + (in.W-dateW)/2, Y: top + under, W: dateW, H: dateH}
	}

	return l, date
}

func (segments) Draw(s ui.Surface, in ui.Rect, r Reading, palette theme.Theme) {
	if _, _, ok := lines(r); !ok {
		plain{}.Draw(s, in, r, palette)
		return
	}

	l, date := segmentsArrange(in, r)

	on := palette.Text
	off := palette.Surface.Blend(palette.Text, segmentGhost)

	for _, c := range l.Cells {
		if c.Ch == ':' {
			drawColon(s, c.At.X, c.At.Y, c.At.H, on)
			continue
		}
		drawDigit(s, c.At, c.Ch, on, off)
	}

	if r.Dated() {
		ui.DrawText(s, date, l.Date.X, l.Date.Y, palette.Muted, palette.Background, r.Date)
	}
}

// runWidth is how wide the whole time comes out at a digit height.
func runWidth(text string, width, gap, height int) int {
	var w int
	for _, c := range text {
		if c == ':' {
			w += int(float64(height)*colonGap) + gap
			continue
		}
		w += width + gap
	}
	return w - gap
}

// segmentsFit is the digit height that fits the box on both axes.
//
// Worked out rather than searched: every part of this face is a fixed fraction of the height, so
// the width it needs is a fixed multiple too, and the largest height that fits is division.
func segmentsFit(in ui.Rect, rows []string) int {
	share := segmentsHeightShare
	if len(rows) > 1 {
		share = segmentsRowShare
	}
	height := max(int(float64(in.H)*share), 1)

	var wide int
	for _, row := range rows {
		wide = max(wide, runWidth(row,
			int(float64(height)*digitAspect), int(float64(height)*digitGap), height))
	}

	if room := int(float64(in.W) * segmentsWidthShare); wide > room {
		height = height * room / wide
	}
	return max(height, 1)
}
