package face

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() { register(config.FaceCards, cards{}) }

// cards is the hour and the minutes on two panels, each with a seam across its middle where the
// flap of a flip clock would fall. What this device showed before we took it over.
//
// The seam is the whole of it. Two rounded rectangles with numbers in them are a readout; the line
// across the middle is what says the number is printed on something that turns over.
type cards struct{}

// The block, against the box.
const (
	cardsWidthShare  = 0.88
	cardsHeightShare = 0.74
)

// A card and what is on it, against the card's own side, so the face is the same at any size.
const (
	cardRound    = 0.11
	cardGap      = 0.055
	digitsOfCard = 0.54
	digitsWide   = 0.74

	// The seam. Thin, and never less than a pixel: a hairline that rounds away is a card with
	// nothing to say it is a card.
	seamThick = 0.007

	// The suffix sits inside the hour card, off its top right corner.
	suffixOfCard = 0.15
	suffixInset  = 0.09

	dateOfCard    = 0.085
	dateGapOfCard = 0.16
)

// cardsLayout is where the pieces land in a box.
type cardsLayout struct {
	// First holds the hour and Second the minutes. Which way round they sit is the box's shape,
	// not the reading's.
	First  ui.Rect
	Second ui.Rect

	// Date is the line under the pair, empty when there is no date.
	Date ui.Rect

	// Side is a card's own side, which is what everything drawn on one is sized against.
	Side int
}

// cardsArrange places the pieces, so that drawing them and saying where they are cannot disagree.
func cardsArrange(in ui.Rect, r Reading) (cardsLayout, *ui.Font) {
	// Side by side when there is width for it and stacked when there is not. The same face turned,
	// rather than a second one: a card is square, and two squares only fit across a portrait panel
	// by being half the size they could be.
	across := in.W >= in.H

	side, gap := cardsFit(in, across)
	date := ui.MustLoad(ui.Regular, max(int(float64(side)*dateOfCard), 1))

	dateW, dateH, under := 0, 0, 0
	if r.Dated() {
		dateW, dateH = date.Measure(r.Date)
		under = int(float64(side) * dateGapOfCard)
	}

	blockW, blockH := side, side*2+gap
	if across {
		blockW, blockH = side*2+gap, side
	}

	left := in.X + (in.W-blockW)/2
	top := in.Y + (in.H-(blockH+under+dateH))/2

	l := cardsLayout{
		First:  ui.Rect{X: left, Y: top, W: side, H: side},
		Second: ui.Rect{X: left, Y: top + side + gap, W: side, H: side},
		Side:   side,
	}
	if across {
		l.Second = ui.Rect{X: left + side + gap, Y: top, W: side, H: side}
	}

	if r.Dated() {
		l.Date = ui.Rect{
			X: in.X + (in.W-dateW)/2,
			Y: top + blockH + under,
			W: dateW,
			H: dateH,
		}
	}

	return l, date
}

func (cards) Draw(s ui.Surface, in ui.Rect, r Reading, palette theme.Theme) {
	hour, minute, ok := lines(r)
	if !ok {
		plain{}.Draw(s, in, r, palette)
		return
	}

	l, date := cardsArrange(in, r)

	drawCard(s, l.First, hour, palette)
	drawCard(s, l.Second, minute, palette)

	if r.Suffix != "" {
		// On the hour's card rather than beside the pair, because the space beside them is what
		// makes the two read as one object and putting anything in it breaks that.
		small := ui.MustLoad(ui.Medium, max(int(float64(l.Side)*suffixOfCard), 1))
		inset := int(float64(l.Side) * suffixInset)

		w, _ := small.Measure(r.Suffix)
		ui.DrawText(s, small, l.First.X+l.First.W-inset-w, l.First.Y+inset,
			palette.Muted, palette.Surface, r.Suffix)
	}

	if r.Dated() {
		ui.DrawText(s, date, l.Date.X, l.Date.Y, palette.Muted, palette.Background, r.Date)
	}
}

// drawCard is one panel: the ground, the seam across it, and the number.
//
// The seam is drawn before the number so a digit crossing the middle sits over it, which is what a
// flap does — the line is behind the printing, not ruled across it.
func drawCard(s ui.Surface, at ui.Rect, text string, palette theme.Theme) {
	ui.FillRounded(s, at, int(float64(at.W)*cardRound), palette.Surface)

	// Off the card towards the text rather than towards the background: on a light theme the
	// background is barely darker than the surface, and the seam that says this is a flap was
	// invisible on the panel.
	thick := max(int(float64(at.H)*seamThick), 1)
	seam := palette.Surface.Blend(palette.Text, 0.10)

	ui.FillRect(s, ui.Rect{X: at.X, Y: at.Y + (at.H-thick)/2, W: at.W, H: thick}, seam)

	font := ui.MustLoad(ui.Bold, fit(ui.Bold, text,
		int(float64(at.W)*digitsWide), int(float64(at.H)*digitsOfCard)))

	w, _ := font.Measure(text)
	digits := font.Ascent() - font.Descent()

	// Centered on the digits themselves. Centering the line box instead sits the number high, because
	// the descent it reserves is space these glyphs do not use.
	ui.DrawText(s, font, at.X+(at.W-w)/2, at.Y+(at.H-digits)/2-(font.Ascent()-digits),
		palette.Text, palette.Surface, text)
}

// cardsFit is how big a card can be, and the gap between the pair.
//
// Bounded by the box on both axes and by the room the date needs under it. Cards are square: the
// reference is square, and a card that stretched to fill whatever was left would stop looking like
// something that flips.
func cardsFit(in ui.Rect, across bool) (side, gap int) {
	// Room for the date is taken off the height before the card is sized, rather than discovered
	// afterwards, so the block cannot be laid out and then found not to fit.
	tall := int(float64(in.H) * cardsHeightShare)
	wide := int(float64(in.W) * cardsWidthShare)

	if across {
		// Two cards and a gap across, one card down.
		side = min(tall, int(float64(wide)/(2+cardGap)))
	} else {
		side = min(wide, int(float64(tall)/(2+cardGap)))
	}

	side = max(side, 1)
	return side, int(float64(side) * cardGap)
}
