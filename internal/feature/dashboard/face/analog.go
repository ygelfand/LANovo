package face

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() {
	register(config.FaceAnalog, analog{})
	register(config.FaceAnalogSeconds, analog{seconds: true})
}

// analog is hands over four numerals, at twelve, three, six and nine.
//
// Four rather than a full ring of them, and no dial behind: what makes a clock readable from a
// doorway is the angle of the hands, and everything else drawn around them is what the eye has to
// look past to see it.
//
// The geometry is in hands.go. This file is where the parts go and what color they are.
type analog struct {
	// seconds is the variant with a second hand, which is the same face and a different clock to
	// have in a room: one is something to glance at and the other is visibly running.
	seconds bool
}

// Ticks says the second hand wants a repaint every second. The plain face does not.
func (a analog) Ticks() bool { return a.seconds }

// The dial, against the box.
const dialShare = 0.78

// Everything else against the dial's own diameter, so the face is the same at any size.
const (
	numeralOfDial = 0.25
	numeralRadius = 0.36

	// Heavier than a wall clock's hands, and the minute reaches past the numerals. On the panel the
	// first cut read as a thin drawing of a clock rather than a clock: with nothing behind them the
	// hands are the whole face, and they have to carry it from across a room.
	hourHandOfDial   = 0.28
	minuteHandOfDial = 0.44
	secondHandOfDial = 0.47
	handWidthOfDial  = 0.058
	handTail         = 0.16
	capOfDial        = 0.075

	// The minute is narrower than the hour and the second narrower again, which is how the three
	// are told apart at a glance rather than by following them to their tips.
	minuteOfHand = 0.70
	secondOfHand = 0.28

	analogDateOfDial = 0.075
	analogDateGap    = 0.10
)

// analogLayout is the dial's parts, as the shapes they are drawn from.
//
// The hands are kept as their polygons rather than as rectangles, so that drawing one and saying
// where it is are the same arithmetic — a hand is a tapered quadrilateral at an angle, and a
// rectangle worked out separately from it would drift.
type analogLayout struct {
	// Second is nil on the variant without one.
	Hour, Minute, Second []ui.Point

	// Cap covers the tails where they cross.
	Cap ui.Rect

	// Date is the line under the dial, empty when there is none.
	Date ui.Rect

	// Where the dial is, which is what the numerals are placed from.
	CX, CY, Dial int
}

// analogArrange places the dial. ok is false for a reading this face cannot show.
func (a analog) arrange(in ui.Rect, r Reading) (analogLayout, *ui.Font, bool) {
	h, m, ok := clock(r)
	if !ok {
		return analogLayout{}, nil, false
	}

	dial := int(float64(min(in.W, in.H)) * dialShare)
	date := ui.MustLoad(ui.Regular, max(int(float64(dial)*analogDateOfDial), 1))

	dateW, dateH, under := 0, 0, 0
	if r.Dated() {
		dateW, dateH = date.Measure(r.Date)
		under = int(float64(dial) * analogDateGap)
	}

	// The dial is centered on what is left once the date has its room, so the hands stay in the
	// middle of the face rather than the middle of the box.
	cx := in.X + in.W/2
	cy := in.Y + (in.H-(dial+under+dateH))/2 + dial/2

	turn := float64(h%12)*30 + float64(m)*0.5
	width := float64(dial) * handWidthOfDial

	pin := max(int(float64(dial)*capOfDial), 2)

	l := analogLayout{
		Hour: hand(cx, cy, int(float64(dial)*hourHandOfDial), int(width), turn),
		Minute: hand(cx, cy, int(float64(dial)*minuteHandOfDial),
			int(width*minuteOfHand), float64(m)*6),
		Cap:  ui.Rect{X: cx - pin/2, Y: cy - pin/2, W: pin, H: pin},
		CX:   cx,
		CY:   cy,
		Dial: dial,
	}

	if a.seconds {
		l.Second = hand(cx, cy, int(float64(dial)*secondHandOfDial),
			int(width*secondOfHand), float64(r.Second)*6)
	}

	if r.Dated() {
		l.Date = ui.Rect{X: in.X + (in.W-dateW)/2, Y: cy + dial/2 + under, W: dateW, H: dateH}
	}

	return l, date, true
}

func (a analog) Draw(s ui.Surface, in ui.Rect, r Reading, palette theme.Theme) {
	l, date, ok := a.arrange(in, r)
	if !ok {
		plain{}.Draw(s, in, r, palette)
		return
	}

	numerals(s, l.CX, l.CY, l.Dial, palette)

	// Hours first, so the minute hand crosses over it: two hands at the same angle have to resolve
	// into one that is clearly in front, or the time is unreadable exactly at the hour.
	//
	// Both in the text color, told apart by length and width the way a clock's are. The accent is
	// saved for the second hand, because the thing that moves is the thing worth coloring, and a
	// colored hour hand beside a colored second hand would be two claims on the same attention.
	ui.FillPolygon(s, l.Hour, palette.Text)
	ui.FillPolygon(s, l.Minute, palette.Text)

	if l.Second != nil {
		ui.FillPolygon(s, l.Second, palette.Accent)
	}

	// The cap covers the tails where they cross, which is what a real one is for.
	ui.FillRounded(s, l.Cap, l.Cap.W/2, palette.Text)

	if r.Dated() {
		ui.DrawText(s, date, l.Date.X, l.Date.Y, palette.Muted, palette.Background, r.Date)
	}
}
