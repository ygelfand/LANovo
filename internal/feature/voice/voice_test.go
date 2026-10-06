package voice

import (
	"slices"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/lib/wake"
)

func models(ids ...string) []wake.Model {
	out := make([]wake.Model, 0, len(ids))
	for _, id := range ids {
		out = append(out, wake.Model{ID: id, Phrase: id})
	}
	return out
}

// What a device listens for when nothing has been chosen decides whether a fresh install can be
// spoken to at all, and it must not come down to which model sorts first.
func TestNothingChosenPreselectsTheDefault(t *testing.T) {
	config.Use(t.TempDir() + "/state.json")
	for name, tc := range map[string]struct {
		installed []string
		want      string
	}{
		"the default is installed":     {[]string{"hey_jarvis", wake.DefaultModel, "hey_mycroft"}, wake.DefaultModel},
		"the default sorts last":       {[]string{"alexa", wake.DefaultModel}, wake.DefaultModel},
		"the default is not installed": {[]string{"hey_jarvis"}, "hey_jarvis"},
		"nothing installed":            {nil, ""},
	} {
		got := chosen(wanted(models(tc.installed...), wakeword.Slots))

		switch {
		case tc.want == "":
			if len(got) != 0 {
				t.Errorf("%s: listening for %v with nothing installed", name, got)
			}
		case len(got) != 1 || got[0] != tc.want:
			t.Errorf("%s: listening for %v, want just %q", name, got, tc.want)
		}
	}
}

// Each slot is paired with its own pipeline, so a slot with nothing in it has to stay empty rather
// than let the slot after it slide up: the answer would come back from the wrong assistant.
func TestAnEmptySlotIsAGapRatherThanAShift(t *testing.T) {
	config.Use(t.TempDir() + "/state.json")
	if err := config.Set().Wake(1).ID("hey_jarvis"); err != nil {
		t.Fatal(err)
	}

	got := wanted(models("hey_jarvis"), wakeword.Slots)
	if !slices.Equal(got, []string{"", "hey_jarvis"}) {
		t.Errorf("slots are %v, want the word left in the slot that names it", got)
	}
	if adv := chosen(got); !slices.Equal(adv, []string{"hey_jarvis"}) {
		t.Errorf("advertised %v, want the gap taken out", adv)
	}
}

// A slot naming a model the device does not have is a slot that cannot hear: claiming it would leave
// it looking armed while nothing is loaded.
func TestASlotNamingAMissingModelIsEmpty(t *testing.T) {
	config.Use(t.TempDir() + "/state.json")
	if err := config.Set().Wake(0).ID("never_installed"); err != nil {
		t.Fatal(err)
	}

	if got := chosen(wanted(models("hey_jarvis"), wakeword.Slots)); len(got) != 0 {
		t.Errorf("advertised %v for a model that is not on the device", got)
	}
}
