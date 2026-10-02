package ui

import "testing"

// A rounded rectangle has to stay inside itself, at every shape and every radius.
//
// The case that was wrong: a radius of exactly half the width. The corner disc on the right is
// centred a radius in from the right edge and sweeps a radius each way, so it reached one pixel
// past the left edge — a mark outside the rectangle, which nothing that repaints the rectangle
// would ever paint over. A pill is drawn at exactly that radius, and so is the tone band down the
// side of a message card.
func TestARoundedRectangleStaysInsideItself(t *testing.T) {
	for _, size := range [][2]int{
		{10, 168}, {168, 10}, {20, 20}, {21, 21}, {40, 8}, {8, 40}, {2, 2}, {1, 9}, {393, 168},
	} {
		w, h := size[0], size[1]

		for _, radius := range []int{1, 2, 3, 5, h / 2, w / 2, max(w, h)} {
			if radius <= 0 {
				continue
			}

			// Room around it, so anything painted outside has somewhere to land and be seen.
			const pad = 8
			img := NewImage(w+2*pad, h+2*pad, black)
			blank := NewImage(w+2*pad, h+2*pad, black)

			r := Rect{X: pad, Y: pad, W: w, H: h}
			FillRounded(img, r, radius, white)

			if painted := Changed(blank, img); !r.Holds(painted) {
				t.Errorf("%dx%d radius %d painted %+v, outside %+v", w, h, radius, painted, r)
			}
		}
	}
}

// It still fills: a fix that stayed inside by drawing nothing would pass the test above.
func TestARoundedRectangleStillFillsItsMiddle(t *testing.T) {
	img := NewImage(60, 60, black)
	FillRounded(img, Rect{X: 10, Y: 10, W: 40, H: 40}, 20, white)

	if got := img.At(30, 30); got != white {
		t.Errorf("the middle is %v, want it filled", got)
	}
}
