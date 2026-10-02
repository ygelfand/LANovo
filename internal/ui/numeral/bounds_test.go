package numeral

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// A run of digits is drawn inside the box it is handed, whichever set and whatever the run.
//
// Worth checking before anything adopts this (#132): a face will hand over a rectangle and trust
// it, and a set that painted outside would leave marks the face never repaints. The colon is the
// one to watch — it is drawn at a full digit's width centred on a narrower cell, so its box
// overhangs by design and the claim is that the overhang is empty.
func TestEverySetStaysInsideTheBoxItIsGiven(t *testing.T) {
	palette := theme.Default()

	runs := []string{
		"12:34",
		"00:00",
		"9:05",
		":30", // leading colon: its box overhangs to the left
		"12:", // trailing colon: to the right
		":",   // nothing but the overhanging cell
		"1",
		"88:88", // the widest digits most sets have
		"11:11", // the narrowest
	}

	boxes := []ui.Rect{
		{W: 600, H: 200},
		{W: 300, H: 300},
		{W: 900, H: 120},
		{W: 61, H: 41},
		{W: 5, H: 5},
	}

	const pad = 16

	for _, set := range Sets {
		for _, run := range runs {
			for _, shape := range boxes {
				at := ui.Rect{X: pad, Y: pad, W: shape.W, H: shape.H}
				w, h := at.X+at.W+pad, at.Y+at.H+pad

				blank := ui.NewImage(w, h, palette.Background)
				img := ui.NewImage(w, h, palette.Background)

				set.Draw(img, run, at, palette.Text, palette.Background)

				if painted := ui.Changed(blank, img); !at.Holds(painted) {
					t.Errorf("%s drawing %q in %dx%d painted %+v, outside %+v",
						set.Name, run, shape.W, shape.H, painted, at)
				}
			}
		}
	}
}

// One digit in its own cell, which is what Draw hands each of them.
func TestEverySetsDigitsStayInsideTheirCell(t *testing.T) {
	palette := theme.Default()

	const pad = 16
	at := ui.Rect{X: pad, Y: pad, W: 120, H: 200}
	w, h := at.X+at.W+pad, at.Y+at.H+pad

	for _, set := range Sets {
		for d := range 10 {
			blank := ui.NewImage(w, h, palette.Background)
			img := ui.NewImage(w, h, palette.Background)

			set.Digit(img, d, at, palette.Text, palette.Background)

			if painted := ui.Changed(blank, img); !at.Holds(painted) {
				t.Errorf("%s digit %d painted %+v, outside %+v", set.Name, d, painted, at)
			}
		}
	}
}

// Measure says what a run needs, and Draw has to fit in it: a face sizes its box from one and
// paints with the other, so the two disagreeing is a clock that overruns whatever it was given.
func TestWhatMeasureSaysHoldsWhatDrawPaints(t *testing.T) {
	palette := theme.Default()

	const pad = 16

	for _, set := range Sets {
		for _, run := range []string{"12:34", "00:00", "9:05", "88:88"} {
			for _, tall := range []int{40, 120, 200} {
				mw, mh := set.Measure(run, tall)

				at := ui.Rect{X: pad, Y: pad, W: mw, H: mh}
				w, h := at.X+at.W+pad, at.Y+at.H+pad

				blank := ui.NewImage(w, h, palette.Background)
				img := ui.NewImage(w, h, palette.Background)

				set.Draw(img, run, at, palette.Text, palette.Background)

				if painted := ui.Changed(blank, img); !at.Holds(painted) {
					t.Errorf("%s %q at %d tall measured %dx%d but painted %+v",
						set.Name, run, tall, mw, mh, painted)
				}
			}
		}
	}
}
