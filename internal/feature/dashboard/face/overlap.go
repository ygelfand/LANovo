package face

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() { register(config.FaceOverlap, overlap{}) }

// overlap is the hour and the minutes as two heavy blocks that run into each other.
//
// Barely legible from across a room and entirely the point: a clock somebody reads once an hour by
// looking properly, rather than one that announces the time whether or not it was asked. Everything
// else here is drawn so it can be read at a glance; this is the one that is not.
type overlap struct{}

// Sized against the box.
const (
	overlapHeightShare = 0.46
	overlapWidthShare  = 0.78

	// bite is how far the minutes are pulled back into the hour, as a fraction of a block's height.
	// Enough that the two read as one shape; not so far that a digit is lost inside another.
	bite = 0.30

	// shove is how far apart the two sit across the box, so the pair runs down the diagonal rather
	// than sitting in a column. Of a block's width.
	shove = 0.18
)

// Sized against the digits.
const (
	overlapSuffixOfLine = 0.20
	overlapDateOfLine   = 0.13
	overlapDateGap      = 0.30
)

func (o overlap) Draw(s ui.Surface, in ui.Rect, r Reading, palette theme.Theme) {
	hour, minute, ok := lines(r)
	if !ok {
		plain{}.Draw(s, in, r, palette)
		return
	}

	l, m := overlapArrange(in, hour, minute, r)

	// The hour goes down first and the minutes over it, so where they cross it is the minutes that
	// survive. Reading the nearer number as the later one is the only thing keeping this a clock.
	ui.DrawText(s, m.line, l.Hour.X, l.Hour.Y, palette.Muted, palette.Background, hour)
	ui.DrawText(s, m.line, l.Minute.X, l.Minute.Y, palette.Text, palette.Background, minute)

	if r.Suffix != "" {
		// In the front color although it belongs to the hour behind. It sits over the hour's last
		// digit, and drawn in the hour's own muted tone it is grey on grey and cannot be read at all.
		ui.DrawText(s, m.small, l.Suffix.X, l.Suffix.Y,
			palette.Text, palette.Background, r.Suffix)
	}

	if r.Dated() {
		ui.DrawText(s, m.date, l.Date.X, l.Date.Y, palette.Muted, palette.Background, r.Date)
	}
}

// overlapLayout is where the pieces land in a box.
type overlapLayout struct {
	Hour   ui.Rect
	Minute ui.Rect

	// Suffix and Date are empty when the reading has neither.
	Suffix ui.Rect
	Date   ui.Rect
}

// overlapArrange places the pieces, so that drawing them and saying where they are cannot disagree.
func overlapArrange(in ui.Rect, hour, minute string, r Reading) (overlapLayout, overlapped) {
	m := layoutOverlap(in, hour, minute, r)

	// The pair is placed as one block: its own width is the two lines plus however far they are
	// shoved apart, and its height is the two less the bite taken out between them.
	wide := m.wide()

	left := in.X + (in.W-wide)/2
	top := in.Y + (in.H-m.tall())/2 - (m.ascent - m.digit)

	l := overlapLayout{
		Hour:   ui.Rect{X: left, Y: top, W: m.hourW, H: m.lineH},
		Minute: ui.Rect{X: left + wide - m.minuteW, Y: top + m.between, W: m.minuteW, H: m.lineH},
	}

	if r.Suffix != "" {
		l.Suffix = ui.Rect{
			X: left + m.hourW - m.suffixW,
			Y: top + (m.ascent - m.digit) - m.small.Descent(),
			W: m.suffixW,
			H: m.suffixH,
		}
	}

	if r.Dated() {
		l.Date = ui.Rect{
			X: max(in.X, min(left+wide-m.dateW, in.X+in.W-m.dateW)),
			Y: top + m.between + m.ascent + m.under,
			W: m.dateW,
			H: m.dateH,
		}
	}

	return l, m
}

// overlapped is the face measured.
type overlapped struct {
	line, small, date *ui.Font

	hourW, minuteW int
	suffixW        int
	dateW, dateH   int

	// lineH is the line box a row of numerals occupies, and suffixH the suffix's.
	lineH   int
	suffixH int

	digit, ascent int

	// between is the drop from the hour's baseline to the minutes', and under the drop to the date.
	between, under int

	// across is how far the minutes sit to the right of the hour.
	across int
}

func (m overlapped) wide() int { return max(m.hourW, m.minuteW) + m.across }

func (m overlapped) tall() int { return m.digit + m.between + m.under + m.dateH }

func layoutOverlap(in ui.Rect, hour, minute string, r Reading) overlapped {
	maxW := int(float64(in.W) * overlapWidthShare)
	maxH := int(float64(in.H) * overlapHeightShare)

	size := min(
		fit(ui.Bold, hour, maxW, maxH),
		fit(ui.Bold, minute, maxW, maxH),
	)

	for range 8 {
		m := measureOverlap(size, hour, minute, r)

		next := size
		if m.wide() > maxW {
			next = min(next, size*maxW/m.wide())
		}
		if m.tall() > in.H {
			next = min(next, size*in.H/m.tall())
		}

		if next == size || size <= 1 {
			return m
		}
		if next >= size {
			next = size - 1
		}
		size = max(next, 1)
	}
	return measureOverlap(size, hour, minute, r)
}

func measureOverlap(size int, hour, minute string, r Reading) overlapped {
	m := overlapped{
		line:  ui.MustLoad(ui.Bold, size),
		small: ui.MustLoad(ui.Medium, max(int(float64(size)*overlapSuffixOfLine), 1)),
		date:  ui.MustLoad(ui.Regular, max(int(float64(size)*overlapDateOfLine), 1)),
	}

	m.hourW, m.lineH = m.line.Measure(hour)
	m.minuteW, _ = m.line.Measure(minute)

	if r.Dated() {
		m.dateW, m.dateH = m.date.Measure(r.Date)
	}
	if r.Suffix != "" {
		m.suffixW, m.suffixH = m.small.Measure(r.Suffix)
	}

	m.ascent = m.line.Ascent()
	m.digit = max(m.ascent-m.line.Descent(), 1)

	// A full block down, less the bite. Positive whatever the bite is set to, so the minutes are
	// always the lower of the two and the face cannot invert into something unreadable.
	m.between = max(m.digit-int(float64(m.digit)*bite), 1)
	m.across = int(float64(max(m.hourW, m.minuteW)) * shove)

	if r.Dated() {
		m.under = int(float64(m.digit) * overlapDateGap)
	}
	return m
}
