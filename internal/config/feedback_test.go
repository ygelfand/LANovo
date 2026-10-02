package config

import "testing"

// None has to be reachable, and it has to be the one somebody finds first: it is what they went
// looking for at two in the morning.
func TestNoneIsOfferedFirst(t *testing.T) {
	got := Chimes()

	if len(got) == 0 {
		t.Fatal("nothing is offered")
	}
	if got[0] != ChimeNone {
		t.Errorf("the first choice is %v, want None", got[0])
	}
}

func TestOnlyNoneIsSilent(t *testing.T) {
	if !ChimeNone.Silent() {
		t.Error("None makes a sound")
	}
	for _, c := range Chimes() {
		if c != ChimeNone && c.Silent() {
			t.Errorf("%v is silent, and only None should be", c)
		}
	}
}

// Home Assistant speaks labels and everything else speaks values, so the round trip has to hold or
// picking one in Home Assistant saves a different one.
func TestEveryChimeRoundTripsThroughItsLabel(t *testing.T) {
	seen := map[string]Chime{}

	for _, c := range Chimes() {
		label := c.Label()
		if had, ok := seen[label]; ok {
			t.Errorf("%v and %v are both shown as %q", had, c, label)
		}
		seen[label] = c

		got, ok := ByLabel(Chimes(), label)
		if !ok {
			t.Errorf("%q does not resolve back to a chime", label)
			continue
		}
		if got != c {
			t.Errorf("%q resolved to %v, want %v", label, got, c)
		}
	}
}

// A device nobody has set makes the sound it always made, rather than starting silent.
func TestADeviceWithNoSettingStillChimes(t *testing.T) {
	c := Defaults()

	if c.Feedback.Chime.Silent() {
		t.Error("a fresh device is silent")
	}
	if c.Feedback.Chime != DefaultChime {
		t.Errorf("a fresh device is set to %v, want %v", c.Feedback.Chime, DefaultChime)
	}
}

// The file is what the device comes back to, and a chime that did not survive a restart would beep
// again the next morning.
func TestTheChimeSurvivesARestart(t *testing.T) {
	path := fresh(t)

	if err := Set().Feedback().Chime(ChimeNone); err != nil {
		t.Fatal(err)
	}

	Use(path)
	if got := Get().Feedback.Chime; got != ChimeNone {
		t.Errorf("came back as %v, want None", got)
	}
}
