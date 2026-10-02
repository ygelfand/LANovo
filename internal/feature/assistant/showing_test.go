package assistant

import (
	"testing"
)

func TestTheReplyIsRevealedByCharacter(t *testing.T) {
	const text = "The kitchen light is on"

	for _, tc := range []struct {
		part float64
		want string
	}{
		{0, ""},
		{-1, ""},
		{1, text},
		{2, text},
	} {
		if got := Revealed(text, tc.part); got != tc.want {
			t.Errorf("at %v: %q, want %q", tc.part, got, tc.want)
		}
	}

	half := Revealed(text, 0.5)
	if len(half) == 0 || len(half) >= len(text) {
		t.Errorf("half way through reads %q", half)
	}
	if text[:len(half)] != half {
		t.Errorf("%q is not the start of %q", half, text)
	}
}

// Cut on a rune boundary, or a multi-byte character is drawn as half of itself — which on this
// panel is a replacement glyph in the middle of a sentence.
func TestARevealNeverCutsARuneInHalf(t *testing.T) {
	const text = "påslaget"

	for i := range 40 {
		got := Revealed(text, float64(i)/40)
		for _, r := range got {
			if r == '�' {
				t.Fatalf("at %d/40 the reveal reads %q", i, got)
			}
		}
	}
}

// Every phase moves as time passes, at whatever the room is doing — including a silent one. This
// is what says the device is listening rather than hung, and it is the whole of what somebody sees
// before they start talking.
//
// The level is held the same across the two frames on purpose. Comparing a quiet frame against a
// loud one proves only that the level changes the drawing, which it obviously does; it was written
// that way first and passed while the wave stood still on the device.
