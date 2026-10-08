package voice

import (
	"slices"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/inference/wake"
	"github.com/ygelfand/libcountertop/pkg/inference/wakeslots"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
)

func models(ids ...string) []wake.Model {
	out := make([]wake.Model, 0, len(ids))
	for _, id := range ids {
		out = append(out, wake.Model{ID: id, Phrase: id})
	}
	return out
}

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
		got := wakeslots.Advertised(wanted(models(tc.installed...), wakeword.Slots))

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

func TestAnEmptySlotIsAGapRatherThanAShift(t *testing.T) {
	config.Use(t.TempDir() + "/state.json")
	if err := config.Set().Wake(1).ID("hey_jarvis"); err != nil {
		t.Fatal(err)
	}

	got := wanted(models("hey_jarvis"), wakeword.Slots)
	if !slices.Equal(got, []string{"", "hey_jarvis"}) {
		t.Errorf("slots are %v, want the word left in the slot that names it", got)
	}
	if adv := wakeslots.Advertised(got); !slices.Equal(adv, []string{"hey_jarvis"}) {
		t.Errorf("advertised %v, want the gap taken out", adv)
	}
}

func TestASlotNamingAMissingModelIsEmpty(t *testing.T) {
	config.Use(t.TempDir() + "/state.json")
	if err := config.Set().Wake(0).ID("never_installed"); err != nil {
		t.Fatal(err)
	}

	if got := wakeslots.Advertised(wanted(models("hey_jarvis"), wakeword.Slots)); len(got) != 0 {
		t.Errorf("advertised %v for a model that is not on the device", got)
	}
}
