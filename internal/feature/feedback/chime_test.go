package feedback

import (
	"path/filepath"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

// A setting changed from the screen has to reach Home Assistant, and one changed from Home
// Assistant has to reach the file. Both go through SetChime, which is the point of it existing.
//
// This is the drift that has already happened three times: a settings row that wrote the config and
// left the entity saying the old value until the device reconnected.
func TestSettingTheChimeTellsBothSides(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	f := Get()

	for _, want := range config.Chimes() {
		f.SetChime(want)

		if got := config.Get().Feedback.Chime; got != want {
			t.Errorf("the file says %q, want %q", got, want)
		}
		if got := f.chime.Get(); got != want.Label() {
			t.Errorf("the entity says %q, want %q", got, want.Label())
		}
	}
}

// The other direction: what Home Assistant sends lands in the file too.
func TestTheEntityCommandSavesTheChime(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	f := Get()

	for _, want := range config.Chimes() {
		f.chime.OnCommand(want.Label())

		if got := config.Get().Feedback.Chime; got != want {
			t.Errorf("%q from Home Assistant saved as %q", want, got)
		}
	}
}

// A label nobody has changes nothing, rather than saving an empty chime that plays silence with no
// way to tell why.
func TestAnUnknownChimeIsIgnored(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	f := Get()
	f.SetChime(config.DefaultChime)

	f.chime.OnCommand("Foghorn")

	if got := config.Get().Feedback.Chime; got != config.DefaultChime {
		t.Errorf("an unknown chime left the setting on %q", got)
	}
}
