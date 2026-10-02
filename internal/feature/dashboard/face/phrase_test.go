package face

import (
	"strings"
	"testing"
)

func TestSay(t *testing.T) {
	tests := []struct {
		hour, minute int
		want         string
	}{
		{6, 0, "six o'clock"},
		{6, 1, "six o'clock"},
		{6, 4, "five past six"},
		{6, 5, "five past six"},
		{6, 10, "ten past six"},
		{6, 15, "quarter past six"},
		{6, 20, "twenty past six"},
		{6, 25, "twenty-five past six"},
		{6, 30, "half past six"},
		{6, 35, "twenty-five to seven"},
		{6, 40, "twenty to seven"},
		{6, 45, "quarter to seven"},
		{6, 50, "ten to seven"},
		{6, 55, "five to seven"},
		{6, 58, "seven o'clock"},

		// The hour rolls over, and twelve is twelve rather than zero.
		{11, 55, "five to twelve"},
		{12, 0, "twelve o'clock"},
		{23, 45, "quarter to twelve"},
		{0, 30, "half past twelve"},
		{0, 0, "twelve o'clock"},
	}

	for _, tt := range tests {
		got := strings.Join(Say(tt.hour, tt.minute), " ")

		if got != tt.want {
			t.Errorf("%02d:%02d is %q, want %q", tt.hour, tt.minute, got, tt.want)
		}
	}
}

// Every minute of the day has to come out as something sayable, since the face has no other way to
// show the time and a blank screen is indistinguishable from a device that has stopped.
func TestEveryMinuteHasWords(t *testing.T) {
	for hour := range 24 {
		for minute := range 60 {
			said := Say(hour, minute)

			if len(said) == 0 {
				t.Fatalf("%02d:%02d says nothing", hour, minute)
			}
			for _, line := range said {
				if strings.TrimSpace(line) == "" {
					t.Fatalf("%02d:%02d has an empty line: %q", hour, minute, said)
				}
			}
		}
	}
}

// Always two lines: a third would make every other phrase smaller to fit the worst one.
func TestThePhraseIsAlwaysTwoLines(t *testing.T) {
	for hour := range 24 {
		for minute := range 60 {
			if got := Say(hour, minute); len(got) != 2 {
				t.Fatalf("%02d:%02d is %d lines: %q", hour, minute, len(got), got)
			}
		}
	}
}

// The phrase must never be more than three minutes from the truth, which is the most the rounding
// to five can cost. A bug in the turn-around is what would break this, not the rounding itself.
func TestThePhraseIsNeverFarFromTheTime(t *testing.T) {
	for hour := range 24 {
		for minute := range 60 {
			said := strings.Join(Say(hour, minute), " ")

			var best int
			var found bool

			for off := -3; off <= 3; off++ {
				at := (hour*60 + minute + off + 24*60) % (24 * 60)
				exact := strings.Join(Say(at/60, at%60), " ")

				if exact == said && (!found || abs(off) < abs(best)) {
					best, found = off, true
				}
			}
			if !found {
				t.Fatalf("%02d:%02d says %q, which is not within three minutes of it",
					hour, minute, said)
			}
		}
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
