package feedback

import (
	"path/filepath"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/config"
)

func TestSettingTheChimeTellsBothSides(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	f := Get()

	for _, want := range config.Chimes() {
		f.SetChime(want)

		if got := config.Get().Feedback.Chime; got != want {
			t.Errorf("the file says %q, want %q", got, want)
		}
		if got := f.Entities()[0].(*esphome.Select).Get(); got != want.Label() {
			t.Errorf("the entity says %q, want %q", got, want.Label())
		}
	}
}

func TestTheEntityCommandSavesTheChime(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	f := Get()

	for _, want := range config.Chimes() {
		f.Entities()[0].(*esphome.Select).OnCommand(want.Label())

		if got := config.Get().Feedback.Chime; got != want {
			t.Errorf("%q from Home Assistant saved as %q", want, got)
		}
	}
}

func TestAnUnknownChimeIsIgnored(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	f := Get()
	f.SetChime(config.DefaultChime)

	f.Entities()[0].(*esphome.Select).OnCommand("Foghorn")

	if got := config.Get().Feedback.Chime; got != config.DefaultChime {
		t.Errorf("an unknown chime left the setting on %q", got)
	}
}
