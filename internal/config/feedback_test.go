package config

import "testing"

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

func TestADeviceWithNoSettingStillChimes(t *testing.T) {
	c := Defaults()

	if c.Feedback.Chime.Silent() {
		t.Error("a fresh device is silent")
	}
	if c.Feedback.Chime != DefaultChime {
		t.Errorf("a fresh device is set to %v, want %v", c.Feedback.Chime, DefaultChime)
	}
}

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
