package numeral

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// The contract every set is held to, so a half-converted one cannot land: eleven glyphs, each
// drawing something, all in one design box.
func TestEverySetIsWholeAndDrawsSomething(t *testing.T) {
	for _, set := range Sets {
		t.Run(set.Name, func(t *testing.T) {
			if set.BoxW <= 0 || set.BoxH <= 0 {
				t.Fatalf("the design box is %dx%d", set.BoxW, set.BoxH)
			}

			for d := range 10 {
				if len(set.Digits[d]) == 0 {
					t.Errorf("%d is missing", d)
					continue
				}
				if !draws(set, string(rune('0'+d))) {
					t.Errorf("%d drew nothing", d)
				}
			}

			if len(set.Colon) == 0 {
				t.Fatal("the colon is missing")
			}
			if !draws(set, ":") {
				t.Error("the colon drew nothing")
			}
		})
	}
}

// Tabular is the whole point: the clock cannot shift as the time changes. What that needs is for
// the width to depend on how many characters there are and not on which ones.
func TestARunIsTheSameWidthWhateverTheTime(t *testing.T) {
	for _, set := range Sets {
		for _, tall := range []int{40, 180, 900} {
			want, h := set.Measure("00:00", tall)

			if h != tall {
				t.Errorf("%s at %d: measured %d tall", set.Name, tall, h)
			}

			// Every minute of the day, since a digit that measured differently would only show at
			// the times it appeared in.
			for _, said := range []string{"11:11", "12:34", "23:59", "88:88", "09:05"} {
				if w, _ := set.Measure(said, tall); w != want {
					t.Errorf("%s at %d: %q measures %d, %q measures %d",
						set.Name, tall, said, w, "00:00", want)
				}
			}
		}
	}
}

// The colon takes less room than a digit, or the clock has a hole in the middle of it.
func TestTheColonIsNarrowerThanADigit(t *testing.T) {
	for _, set := range Sets {
		const tall = 180

		digit := set.Wide(tall)

		with, _ := set.Measure("0:0", tall)
		without, _ := set.Measure("00", tall)

		colon := with - without
		if colon <= 0 {
			t.Errorf("%s: the colon takes %d", set.Name, colon)
			continue
		}
		if colon >= digit {
			t.Errorf("%s: the colon takes %d, a digit takes %d", set.Name, colon, digit)
		}
	}
}

// Wide and Tall are each other's inverse, within the rounding of whole pixels. A face bounded by
// width and one bounded by height have to arrive at the same digit.
func TestWideAndTallAgree(t *testing.T) {
	for _, set := range Sets {
		for tall := 20; tall <= 900; tall += 37 {
			back := set.Tall(set.Wide(tall))

			// Two integer divisions, so a pixel either way is the floor of what is achievable.
			if diff := back - tall; diff > 2 || diff < -2 {
				t.Errorf("%s: %d tall gives %d wide gives %d tall", set.Name, tall, set.Wide(tall), back)
			}
		}
	}
}

// A run is laid out inside the box it is given and does not spill out of it.
func TestARunStaysInItsBox(t *testing.T) {
	palette := theme.Default()
	edge := theme.Color{R: 255, G: 0, B: 255}

	for _, set := range Sets {
		img := ui.NewImage(400, 240, edge)

		in := ui.Rect{X: 40, Y: 30, W: 320, H: 180}
		ui.FillRect(img, in, palette.Background)

		set.Draw(img, "12:34", in, palette.Text, palette.Background)

		for y := range 240 {
			for x := range 400 {
				if in.Contains(x, y) {
					continue
				}
				if img.At(x, y) != edge {
					t.Fatalf("%s painted at %d,%d, outside %v", set.Name, x, y, in)
				}
			}
		}
	}
}

// Digits next to each other have to stay apart, or 11 reads as one wide shape. Checked down the
// seam between two cells rather than by eye.
func TestNeighbouringDigitsDoNotTouch(t *testing.T) {
	palette := theme.Default()

	for _, set := range Sets {
		img := ui.NewImage(400, 240, palette.Background)

		in := ui.Rect{X: 0, Y: 0, W: 400, H: 240}
		set.Draw(img, "11", in, palette.Text, palette.Background)

		seam := in.W / 2
		for y := range 240 {
			if img.At(seam, y) != palette.Background {
				t.Errorf("%s: the two digits meet at the seam, %d down", set.Name, y)
				break
			}
		}
	}
}

// A character the set has no glyph for is skipped, not drawn as something else and not a panic.
func TestUnknownCharactersAreSkipped(t *testing.T) {
	palette := theme.Default()
	set := Default()

	img := ui.NewImage(200, 120, palette.Background)
	set.Draw(img, "ab", ui.Rect{W: 200, H: 120}, palette.Text, palette.Background)

	for y := range 120 {
		for x := range 200 {
			if img.At(x, y) != palette.Background {
				t.Fatalf("letters drew something at %d,%d", x, y)
			}
		}
	}
}

func TestAnUnknownSetIsNotOne(t *testing.T) {
	if _, ok := ByName("copperplate"); ok {
		t.Error("a set nobody has was found")
	}
	if Default().Name != DefaultName {
		t.Errorf("the default is %q, want %q", Default().Name, DefaultName)
	}
}

// draws reports whether one character puts anything on the panel.
func draws(set Set, text string) bool {
	palette := theme.Default()

	img := ui.NewImage(120, 200, palette.Background)
	set.Draw(img, text, ui.Rect{W: 120, H: 200}, palette.Text, palette.Background)

	for y := range 200 {
		for x := range 120 {
			if img.At(x, y) != palette.Background {
				return true
			}
		}
	}
	return false
}
