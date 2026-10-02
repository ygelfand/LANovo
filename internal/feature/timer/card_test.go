package timer

import (
	"testing"
	"time"
)

// What the countdown reads. Rounded up, because a five minute timer that says 4:59 the instant it
// starts looks like it lost a second before anyone blinked.
func TestTheCountdownReads(t *testing.T) {
	for _, tc := range []struct {
		left time.Duration
		want string
	}{
		{5 * time.Minute, "5:00"},
		{5*time.Minute - time.Millisecond, "5:00"},
		{4*time.Minute + 59*time.Second, "4:59"},
		{59 * time.Second, "0:59"},
		{time.Millisecond, "0:01"},
		{0, "0:00"},
		{-time.Second, "0:00"},
		{time.Hour, "1:00:00"},
		{90 * time.Minute, "1:30:00"},
		{25 * time.Hour, "25:00:00"},
	} {
		if got := Remaining(tc.left); got != tc.want {
			t.Errorf("%v reads %q, want %q", tc.left, got, tc.want)
		}
	}
}

// The panel is only repainted when what it says changes. The countdown is looked at four times a
// second and only moves once, so a timer with an hour left must not wake the driver 14,400 times.
func TestOnlyASecondChangingIsWorthRepainting(t *testing.T) {
	at := func(left time.Duration) string {
		return Card{Showing: true, Name: "Pasta", Left: left, Of: time.Hour}.key()
	}

	if a, b := at(time.Hour), at(time.Hour-250*time.Millisecond); a != b {
		t.Errorf("a quarter second apart reads differently: %q and %q", a, b)
	}
	if a, b := at(time.Hour), at(time.Hour-time.Second); a == b {
		t.Errorf("a second apart reads the same: %q", a)
	}
}

// Nothing showing has no key at all, so the card coming and going is a change like any other and
// the release is not skipped because the last frame happened to match.
func TestNothingShowingIsItsOwnState(t *testing.T) {
	if got := (Card{}).key(); got != "" {
		t.Errorf("a card that is not showing keys as %q", got)
	}
	if (Card{Showing: true}).key() == "" {
		t.Error("a showing card keys as nothing")
	}
}

// Ringing is a different thing to look at from counting down, even at the same moment on the clock.
func TestRingingIsNotTheSameFrameAsCountingDown(t *testing.T) {
	counting := Card{Showing: true, Left: time.Second}
	ringing := Card{Showing: true, Left: time.Second, Ringing: true}

	if counting.key() == ringing.key() {
		t.Error("a ringing timer draws the same as one still counting")
	}
}
