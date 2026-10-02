package face

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() { register(config.FacePlain, plain{}) }

// plain is the time across the middle with the date under it, which is what the device has always
// drawn.
type plain struct{}

// How much of the box the time takes.
//
// The time is bounded by both: a tall box would otherwise size it off the height and run it off the
// sides, which is what portrait does to a clock sized on height alone.
const (
	timeHeightShare = 0.34
	timeWidthShare  = 0.86
)

// The rest is sized against the time rather than the box, so the face keeps its proportions at any
// size.
const (
	suffixOfTime = 0.24
	dateOfTime   = 0.20
	gapOfTime    = 0.22
	suffixGap    = 0.35
)

// layout is where each part of the plain face goes.
type layout struct {
	Time   ui.Rect
	Suffix ui.Rect
	Date   ui.Rect
}

// fonts are the three sizes one reading is drawn in.
type fonts struct {
	Time   *ui.Font
	Suffix *ui.Font
	Date   *ui.Font
}

func (plain) Draw(s ui.Surface, in ui.Rect, r Reading, palette theme.Theme) {
	l, f := arrange(r, in)

	ui.DrawText(s, f.Time, l.Time.X, l.Time.Y, palette.Text, palette.Background, r.Time)
	if r.Suffix != "" {
		ui.DrawText(s, f.Suffix, l.Suffix.X, l.Suffix.Y, palette.Muted, palette.Background, r.Suffix)
	}
	if r.Dated() {
		ui.DrawText(s, f.Date, l.Date.X, l.Date.Y, palette.Muted, palette.Background, r.Date)
	}
}

// arrange places the reading in a box, centered as a block rather than each line on its own: the date
// sits under the time, and the two move together.
func arrange(r Reading, in ui.Rect) (layout, fonts) {
	maxH := int(float64(in.H) * timeHeightShare)
	maxW := int(float64(in.W) * timeWidthShare)

	size := fit(ui.Bold, r.Time+r.Suffix, maxW, maxH)
	f := fonts{
		Time:   ui.MustLoad(ui.Bold, size),
		Suffix: ui.MustLoad(ui.Medium, int(float64(size)*suffixOfTime)),
		Date:   ui.MustLoad(ui.Regular, int(float64(size)*dateOfTime)),
	}

	timeW, timeH := f.Time.Measure(r.Time)

	dateW, dateH := 0, 0
	if r.Dated() {
		dateW, dateH = f.Date.Measure(r.Date)
	}

	suffixW, suffixH := 0, 0
	if r.Suffix != "" {
		suffixW, suffixH = f.Suffix.Measure(r.Suffix)

		// The suffix sits beside the time, so the pair is centered together.
		suffixW += int(float64(suffixH) * suffixGap)
	}

	between := 0
	if r.Dated() {
		between = int(float64(timeH) * gapOfTime)
	}

	top := in.Y + (in.H-(timeH+between+dateH))/2
	timeX := in.X + (in.W-(timeW+suffixW))/2

	l := layout{
		Time: ui.Rect{X: timeX, Y: top, W: timeW, H: timeH},
		Date: ui.Rect{X: in.X + (in.W-dateW)/2, Y: top + timeH + between, W: dateW, H: dateH},
	}
	if r.Suffix != "" {
		// Sat on the time's baseline rather than its top, which is where an eye expects it.
		l.Suffix = ui.Rect{
			X: timeX + timeW + int(float64(suffixH)*suffixGap),
			Y: top + f.Time.Ascent() - f.Suffix.Ascent(),
			W: suffixW - int(float64(suffixH)*suffixGap),
			H: suffixH,
		}
	}
	return l, f
}
