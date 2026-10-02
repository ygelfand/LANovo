package face

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() { register(config.FaceStack, stack{}) }

// stack is the hour over the minutes, as large as the box allows.
//
// The panel is 1200 by 1920 and stands on its end, where a single line of time is sized by the
// width and leaves most of the height empty. Two lines are sized by the height instead, which is
// the dimension this device has to spare.
type stack struct{}

// How much of the box one line of the stack takes, before the height of the whole block is checked.
const (
	lineHeightShare = 0.34
	lineWidthShare  = 0.80
)

// Sized against the digits rather than the box, so the face keeps its proportions at any size.
const (
	suffixOfLine = 0.22
	dateOfLine   = 0.13

	// Tight, because two lines of the same size read as one block only if the gap between them is
	// small against the digits. The date sits further off, so it is not read as a third line of
	// clock.
	lineGap = 0.06
	dateGap = 0.34

	// The suffix's own gap from the hour beside it.
	suffixGapOfLine = 0.18
)

// stacked is the whole face measured, so placing it and checking it fits are the same arithmetic.
//
// Everything is measured against digit, the height of the digits themselves, rather than the line
// box the font reports. The box carries a descent that digits do not use, and spacing the lines by
// it leaves a band between them that looks like a mistake.
type stacked struct {
	line, small, date *ui.Font

	hourW, minuteW int
	suffixW        int
	dateW, dateH   int

	// lineH is the line box the two rows of numerals occupy, and suffixH the suffix's. Both are
	// what the pieces cover rather than what they are spaced by, which is digit.
	lineH   int
	suffixH int

	// digit is how tall the numerals are, and ascent is where their baseline sits in the line box.
	digit, ascent int

	// between is baseline to baseline, and under is the drop to the top of the date.
	between, under int
}

// tall is the height of the ink, from the top of the digits to the bottom of the date, which is
// what has to fit the box.
func (s stacked) tall() int { return s.digit + s.between + s.under + s.dateH }

// wide is how much width the face needs, which is not the width of its widest line.
//
// The digits stay centered in the box and the suffix hangs off their right, so the room the suffix
// takes has to be left on both sides or the block is centered on paper and off center on the screen.
// Counting it twice is what keeps the two agreeing.
//
// The suffix is also measured in the font it is drawn in. Measuring it in the hour's font instead
// costs the whole face a third of its size, because four large glyphs do not fit where two do.
func (s stacked) wide() int {
	return max(s.hourW+2*(s.gap()+s.suffixW), s.minuteW)
}

// gap is the space between the hour and its suffix, and zero when there is no suffix.
func (s stacked) gap() int {
	if s.suffixW == 0 {
		return 0
	}
	return int(float64(s.digit) * suffixGapOfLine)
}

func (stack) Draw(s ui.Surface, in ui.Rect, r Reading, palette theme.Theme) {
	hour, minute, ok := lines(r)
	if !ok {
		// Not a time this face knows how to split. The plain one takes anything.
		plain{}.Draw(s, in, r, palette)
		return
	}

	l, m := stackArrange(in, hour, minute, r)

	ui.DrawText(s, m.line, l.Hour.X, l.Hour.Y, palette.Text, palette.Background, hour)
	ui.DrawText(s, m.line, l.Minute.X, l.Minute.Y, palette.Text, palette.Background, minute)

	if r.Suffix != "" {
		ui.DrawText(s, m.small, l.Suffix.X, l.Suffix.Y,
			palette.Muted, palette.Background, r.Suffix)
	}

	if r.Dated() {
		ui.DrawText(s, m.date, l.Date.X, l.Date.Y, palette.Muted, palette.Background, r.Date)
	}
}

// stackLayout is where the pieces land in a box.
type stackLayout struct {
	Hour   ui.Rect
	Minute ui.Rect

	// Suffix and Date are empty when the reading has neither.
	Suffix ui.Rect
	Date   ui.Rect
}

// stackArrange places the pieces, so that drawing them and saying where they are cannot disagree.
func stackArrange(in ui.Rect, hour, minute string, r Reading) (stackLayout, stacked) {
	m := layoutStack(in, hour, minute, r)

	// Centered on the digits rather than on the whole line, so the suffix hangs off the side and the
	// two lines of numerals stay in the middle of the box where an eye looks for them.
	left := in.X + (in.W-max(m.hourW, m.minuteW))/2

	// The box is placed so the digits land where the block should start, rather than where the
	// font's own empty space above them would put it.
	top := in.Y + (in.H-m.tall())/2 - (m.ascent - m.digit)

	l := stackLayout{
		Hour:   ui.Rect{X: left, Y: top, W: m.hourW, H: m.lineH},
		Minute: ui.Rect{X: left, Y: top + m.between, W: m.minuteW, H: m.lineH},
	}

	if r.Suffix != "" {
		// At the top of the hour rather than on its baseline. On the baseline it lands in the gap
		// between the two lines and reads as belonging to the minutes under it, which is the one
		// thing it must not say.
		l.Suffix = ui.Rect{
			X: left + m.hourW + m.gap(),
			Y: top + (m.ascent - m.digit) - m.small.Descent(),
			W: m.suffixW,
			H: m.suffixH,
		}
	}

	if r.Dated() {
		// The date hangs off the lower baseline, and is pulled back in when a long one would run
		// past the edge: aligning it to the stack is the nicety and staying in the box is not.
		l.Date = ui.Rect{
			X: max(in.X, min(left, in.X+in.W-m.dateW)),
			Y: top + m.between + m.ascent + m.under,
			W: m.dateW,
			H: m.dateH,
		}
	}

	return l, m
}

// layoutStack measures the face at the largest size that fits the box.
//
// Width is bounded first and then the ink is checked against the box. Sizing two lines off a share
// of the height and trusting it to fit is what put the date off the bottom the first time: the font
// reports a line box larger than the size it was asked for.
func layoutStack(in ui.Rect, hour, minute string, r Reading) stacked {
	maxW := int(float64(in.W) * lineWidthShare)
	maxH := int(float64(in.H) * lineHeightShare)

	// A first guess from the digits alone, then measured whole: everything scales with the size, so
	// overrunning by some fraction says what to multiply the size by.
	size := min(
		fit(ui.Bold, hour, maxW, maxH),
		fit(ui.Bold, minute, maxW, maxH),
	)

	for range 8 {
		m := measureStack(size, hour, minute, r)

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
	return measureStack(size, hour, minute, r)
}

func measureStack(size int, hour, minute string, r Reading) stacked {
	m := stacked{
		line:  ui.MustLoad(ui.Bold, size),
		small: ui.MustLoad(ui.Medium, max(int(float64(size)*suffixOfLine), 1)),
		date:  ui.MustLoad(ui.Regular, max(int(float64(size)*dateOfLine), 1)),
	}

	m.hourW, m.lineH = m.line.Measure(hour)
	m.minuteW, _ = m.line.Measure(minute)

	if r.Dated() {
		m.dateW, m.dateH = m.date.Measure(r.Date)
	}

	if r.Suffix != "" {
		m.suffixW, m.suffixH = m.small.Measure(r.Suffix)
	}

	// The digits reach from the baseline up by about the ascent less the descent, which is close
	// enough to a cap height for spacing and needs no metric the font does not already give.
	m.ascent = m.line.Ascent()
	m.digit = max(m.ascent-m.line.Descent(), 1)

	m.between = m.digit + int(float64(m.digit)*lineGap)
	if r.Dated() {
		m.under = int(float64(m.digit) * dateGap)
	}

	return m
}
