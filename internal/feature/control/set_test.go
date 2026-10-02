package control

import (
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

// The value readers are exercised against the real settings table; the setters are not, since they
// reach the dashboard and the panel. What is worth holding here is the parsing, which is where a
// harness command goes wrong quietly.

func TestChooseTakesTheLabelOrTheStoredName(t *testing.T) {
	for _, want := range config.Faces() {
		for _, said := range []string{want.Label(), string(want), strings.ToUpper(string(want))} {
			var got config.Face
			if err := choose(config.Faces(), func(f config.Face) { got = f })(said); err != nil {
				t.Errorf("%q: %v", said, err)
				continue
			}
			if got != want {
				t.Errorf("%q chose %q, want %q", said, got, want)
			}
		}
	}
}

// A value that is not one of the options has to say so, and say what the options are. A harness
// that accepted anything would leave a sweep running against a device it never changed.
func TestChooseRefusesWhatIsNotAnOption(t *testing.T) {
	var called bool
	err := choose(config.Faces(), func(config.Face) { called = true })("sundial")

	if err == nil {
		t.Fatal("a face that does not exist was accepted")
	}
	if called {
		t.Error("the setter ran anyway")
	}
	if !strings.Contains(err.Error(), "Analog") {
		t.Errorf("the error does not list the options: %v", err)
	}
}

func TestToggleReadsTheUsualWords(t *testing.T) {
	for said, want := range map[string]bool{
		"on": true, "true": true, "yes": true, "1": true, "ON": true,
		"off": false, "false": false, "no": false, "0": false,
	} {
		var got bool
		if err := toggle(func(v bool) { got = v })(said); err != nil {
			t.Errorf("%q: %v", said, err)
			continue
		}
		if got != want {
			t.Errorf("%q read as %v, want %v", said, got, want)
		}
	}
}

func TestToggleRefusesAnythingElse(t *testing.T) {
	if err := toggle(func(bool) {})("maybe"); err == nil {
		t.Error("maybe was accepted as a switch")
	}
}

// Out of range is refused rather than clamped: a harness that asked for 150 and silently got 100
// is a test that passed for the wrong reason.
func TestNumberRefusesOutOfRange(t *testing.T) {
	for _, said := range []string{"-1", "101", "howbright"} {
		var called bool
		if err := number(0, 100, func(int) { called = true })(said); err == nil {
			t.Errorf("%q was accepted", said)
		}
		if called {
			t.Errorf("%q ran the setter", said)
		}
	}

	var got int
	if err := number(0, 100, func(v int) { got = v })("76"); err != nil {
		t.Fatalf("76: %v", err)
	}
	if got != 76 {
		t.Errorf("76 read as %d", got)
	}
}

// Every setting has to be readable, or listing them panics on a device somebody is using.
func TestEverySettingReportsItsValue(t *testing.T) {
	cfg := config.Defaults()

	seen := map[string]bool{}
	for _, s := range settings() {
		if s.name == "" {
			t.Error("a setting has no name")
		}
		if seen[s.name] {
			t.Errorf("%q is listed twice", s.name)
		}
		seen[s.name] = true

		if s.says == nil || s.use == nil {
			t.Fatalf("%q is not wired up", s.name)
			continue
		}
		if s.says(cfg) == "" {
			t.Errorf("%q reads as nothing on a fresh device", s.name)
		}
	}
}

// A name nobody has is an error that points at how to find the right one, rather than silence.
func TestSetRefusesASettingThatDoesNotExist(t *testing.T) {
	_, err := set([]string{"clock.colour"})

	if err == nil {
		t.Fatal("an unknown setting was accepted")
	}
	if !strings.Contains(err.Error(), "no such setting") {
		t.Errorf("unhelpful error: %v", err)
	}
}
