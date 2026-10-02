package face

import (
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// The geometry of a dial, kept apart from the face that draws one: this is trigonometry and the
// face is a look, and the two change for different reasons.

// hand is one hand as a tapered quadrilateral: wide at the tail, narrower at the tip, and carried a
// little past the center so it looks pinned rather than balanced on a point.
//
// Degrees clockwise from twelve, which is how a clock is described and not how the maths works, so
// the conversion happens here once instead of at each caller.
func hand(cx, cy, length, width int, deg float64) []ui.Point {
	dx, dy := heading(deg)
	px, py := -dy, dx

	tipX, tipY := float64(cx)+dx*float64(length), float64(cy)+dy*float64(length)

	tail := float64(length) * handTail
	tailX, tailY := float64(cx)-dx*tail, float64(cy)-dy*tail

	half := float64(width) / 2
	tip := half * 0.55

	return []ui.Point{
		{X: int(tailX + px*half), Y: int(tailY + py*half)},
		{X: int(tipX + px*tip), Y: int(tipY + py*tip)},
		{X: int(tipX - px*tip), Y: int(tipY - py*tip)},
		{X: int(tailX - px*half), Y: int(tailY - py*half)},
	}
}

// numerals puts 12, 3, 6 and 9 around the center, each centered on its own position rather than on
// its line box: a numeral placed by its box sits low, because the box reserves a descent the digits
// do not use.
func numerals(s ui.Surface, cx, cy, dial int, palette theme.Theme) {
	font := ui.MustLoad(ui.Bold, max(int(float64(dial)*numeralOfDial), 1))
	radius := float64(dial) * numeralRadius

	digits := font.Ascent() - font.Descent()

	for i, text := range []string{"12", "3", "6", "9"} {
		dx, dy := heading(float64(i) * 90)

		x := float64(cx) + radius*dx
		y := float64(cy) + radius*dy

		w, _ := font.Measure(text)
		ui.DrawText(s, font, int(x)-w/2, int(y)-digits/2-(font.Ascent()-digits),
			palette.Text, palette.Background, text)
	}
}

// heading is the unit vector at an angle clockwise from twelve, in screen coordinates where down is
// positive.
func heading(deg float64) (dx, dy float64) {
	rad := deg * math.Pi / 180
	return math.Sin(rad), -math.Cos(rad)
}
