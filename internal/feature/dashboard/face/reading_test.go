package face

import (
	"testing"
	"time"
)

func at(hour, minute int) time.Time {
	return time.Date(2026, time.September, 19, hour, minute, 0, 0, time.UTC)
}

func TestRead(t *testing.T) {
	tests := []struct {
		name       string
		at         time.Time
		twentyFour bool
		wantTime   string
		wantSuffix string
	}{
		{"morning, 24 hour", at(9, 5), true, "09:05", ""},
		{"afternoon, 24 hour", at(15, 30), true, "15:30", ""},
		{"midnight, 24 hour", at(0, 0), true, "00:00", ""},
		{"morning, 12 hour", at(9, 5), false, "9:05", "AM"},
		{"afternoon, 12 hour", at(15, 30), false, "3:30", "PM"},
		{"midnight, 12 hour", at(0, 0), false, "12:00", "AM"},
		{"noon, 12 hour", at(12, 0), false, "12:00", "PM"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Read(tt.at, tt.twentyFour)

			if got.Time != tt.wantTime {
				t.Errorf("time = %q, want %q", got.Time, tt.wantTime)
			}
			if got.Suffix != tt.wantSuffix {
				t.Errorf("suffix = %q, want %q", got.Suffix, tt.wantSuffix)
			}
		})
	}
}

// A 24 hour clock has no am or pm to show, and leaving one on would be read as the wrong time of
// day.
func TestTwentyFourHourHasNoSuffix(t *testing.T) {
	for hour := range 24 {
		if got := Read(at(hour, 0), true); got.Suffix != "" {
			t.Errorf("%02d:00 carried the suffix %q", hour, got.Suffix)
		}
	}
}

func TestDate(t *testing.T) {
	if got := Read(at(9, 0), true).Date; got != "Saturday, 19 September" {
		t.Errorf("date = %q, want %q", got, "Saturday, 19 September")
	}
}

// The reading is what decides whether to repaint, so two different minutes must not look the same.
func TestTheReadingChangesWithTheMinute(t *testing.T) {
	a := Read(at(9, 5), true).String()
	b := Read(at(9, 6), true).String()

	if a == b {
		t.Errorf("9:05 and 9:06 both read %q", a)
	}
}

// Within a minute nothing changes, which is what stops the panel being redrawn every second.
func TestTheReadingIsSteadyWithinAMinute(t *testing.T) {
	a := Read(time.Date(2026, time.September, 19, 9, 5, 0, 0, time.UTC), true).String()
	b := Read(time.Date(2026, time.September, 19, 9, 5, 59, 0, time.UTC), true).String()

	if a != b {
		t.Errorf("the same minute read %q then %q", a, b)
	}
}

// Dropping the date has to change the reading, or the dashboard will not notice the setting moved
// and will leave the old screen up.
func TestUndatedIsADifferentReading(t *testing.T) {
	r := Read(at(9, 5), true)

	if !r.Dated() {
		t.Fatal("a fresh reading has no date")
	}
	if r.Undated().Dated() {
		t.Error("an undated reading still has one")
	}
	if r.String() == r.Undated().String() {
		t.Error("dropping the date left the reading looking the same")
	}
	if r.Undated().Time != r.Time {
		t.Error("dropping the date changed the time")
	}
}

// The faces that stack or box the pair only look right if both are the same width.
func TestLinesPadTheHour(t *testing.T) {
	tests := []struct {
		name       string
		at         int
		twentyFour bool
		hour       string
		minute     string
	}{
		{"single digit, 12 hour", 7, false, "07", "04"},
		{"double digit, 12 hour", 15, false, "03", "04"},
		{"single digit, 24 hour", 7, true, "07", "04"},
		{"double digit, 24 hour", 15, true, "15", "04"},
		{"midnight, 12 hour", 0, false, "12", "04"},
		{"midnight, 24 hour", 0, true, "00", "04"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hour, minute, ok := lines(Read(at(tt.at, 4), tt.twentyFour))

			if !ok {
				t.Fatal("the reading did not split")
			}
			if hour != tt.hour || minute != tt.minute {
				t.Errorf("split to %q over %q, want %q over %q", hour, minute, tt.hour, tt.minute)
			}
			if len(hour) != len(minute) {
				t.Errorf("%q and %q are different lengths, so the block is ragged", hour, minute)
			}
		})
	}
}

// Anything that is not a time falls through to a face that can draw it, rather than drawing half
// of one.
func TestSomethingWithNoColonDoesNotSplit(t *testing.T) {
	if _, _, ok := lines(Reading{Time: "soon"}); ok {
		t.Error("a reading with no colon split anyway")
	}
	if _, _, ok := clock(Reading{Time: "soon"}); ok {
		t.Error("a reading with no colon gave numbers anyway")
	}
}

// The analog and words faces need numbers, and they take them from the same strings the other faces
// draw so every face is showing the same minute.
func TestClockReadsTheReading(t *testing.T) {
	tests := []struct {
		at           int
		twentyFour   bool
		hour, minute int
	}{
		{7, true, 7, 4},
		{7, false, 7, 4},
		{15, true, 15, 4},
		{15, false, 3, 4},
		{0, true, 0, 4},
		{0, false, 12, 4},
	}

	for _, tt := range tests {
		hour, minute, ok := clock(Read(at(tt.at, 4), tt.twentyFour))

		if !ok {
			t.Errorf("%02d:04 did not read", tt.at)
			continue
		}
		if hour != tt.hour || minute != tt.minute {
			t.Errorf("%02d:04 read as %d:%d, want %d:%d", tt.at, hour, minute, tt.hour, tt.minute)
		}
	}
}
