package face

import (
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Only the face with a second hand asks to be redrawn every second. A face that ticked by accident
// would cost the panel a whole-screen composite a second for a picture that did not change.
func TestOnlyTheSecondHandTicks(t *testing.T) {
	for _, name := range config.Faces() {
		want := name == config.FaceAnalogSeconds

		if got := Ticks(name); got != want {
			t.Errorf("%q ticks = %v, want %v", name, got, want)
		}
	}
}

// The second is on every reading whether or not anything draws it, and out of String, so the
// dashboard's own comparison does not see a change sixty times a minute.
func TestTheSecondIsCarriedButNotSaid(t *testing.T) {
	at := func(s int) time.Time {
		return time.Date(2026, time.September, 20, 8, 2, s, 0, time.UTC)
	}

	first, second := Read(at(3), true), Read(at(44), true)

	if first.Second != 3 || second.Second != 44 {
		t.Errorf("the seconds read as %d and %d, want 3 and 44", first.Second, second.Second)
	}
	if first.String() != second.String() {
		t.Errorf("the same minute reads as %q then %q", first, second)
	}
}

// The second hand has to move with the second, which is the only thing that changes between those
// two readings.
func TestTheSecondHandMoves(t *testing.T) {
	at := func(s int) time.Time {
		return time.Date(2026, time.September, 20, 8, 2, s, 0, time.UTC)
	}

	box := ui.Rect{W: 600, H: 600}
	palette := theme.Default()

	shot := func(s int) int {
		img := ui.NewImage(box.W, box.H, palette.Background)
		Of(config.FaceAnalogSeconds).Draw(img, box, Read(at(s), true), palette)

		var lit int
		for y := range box.H {
			for x := range box.W {
				if img.At(x, y) == palette.Accent {
					lit += x * y
				}
			}
		}
		return lit
	}

	if shot(0) == shot(30) {
		t.Error("the second hand is in the same place at :00 and :30")
	}

	// And the face without one does not move, however the second changes.
	plainShot := func(s int) string {
		img := ui.NewImage(box.W, box.H, palette.Background)
		Of(config.FaceAnalog).Draw(img, box, Read(at(s), true), palette)

		var out []byte
		for y := range box.H {
			for x := range box.W {
				if img.At(x, y) != palette.Background {
					out = append(out, byte(x), byte(y))
				}
			}
		}
		return string(out)
	}

	if plainShot(0) != plainShot(30) {
		t.Error("the face with no second hand changed with the second")
	}
}
