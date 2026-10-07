package dashboard

import (
	"path/filepath"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/config"
)

// Every setting the dashboard owns, and the three things that have to stay in step: the setter the
// screen calls, the file, and the entity Home Assistant reads.
//
// Wired by hand three times over, and it has drifted three times — a settings row that wrote the
// file and left the entity saying the old value until the device reconnected. These hold the
// property rather than the wiring, so a setting added without one of the three fails here.
//
// Built directly rather than through Get, which listens to the screen and the accelerometer. The
// entities are all that is under test.
func dash() *Dashboard {
	d := &Dashboard{}
	d.build()
	return d
}

// said is what a select entity is showing.
func said(t *testing.T, e *esphome.Select) string {
	t.Helper()
	return e.Get()
}

func TestEverySelectReachesBothSides(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	d := dash()

	t.Run("face", func(t *testing.T) {
		for _, want := range config.Faces() {
			d.SetFace(want)

			if got := config.Get().Clock.Face; got != want {
				t.Errorf("the file says %q, want %q", got, want)
			}
			if got := said(t, d.face); got != want.Label() {
				t.Errorf("the entity says %q, want %q", got, want.Label())
			}
		}
	})

	t.Run("position", func(t *testing.T) {
		for _, want := range config.Positions() {
			d.SetPosition(want)

			if got := config.Get().Clock.Position; got != want {
				t.Errorf("the file says %q, want %q", got, want)
			}
			if got := said(t, d.place); got != want.Label() {
				t.Errorf("the entity says %q, want %q", got, want.Label())
			}
		}
	})

	t.Run("size", func(t *testing.T) {
		for _, want := range config.Sizes() {
			d.SetSize(want)

			if got := config.Get().Clock.Size; got != want {
				t.Errorf("the file says %q, want %q", got, want)
			}
			if got := said(t, d.size); got != want.Label() {
				t.Errorf("the entity says %q, want %q", got, want.Label())
			}
		}
	})

	t.Run("color", func(t *testing.T) {
		for _, want := range config.Inks() {
			d.SetInk(want)

			if got := config.Get().Clock.Ink; got != want {
				t.Errorf("the file says %q, want %q", got, want)
			}
			if got := said(t, d.ink); got != want.Label() {
				t.Errorf("the entity says %q, want %q", got, want.Label())
			}
		}
	})

	t.Run("hours", func(t *testing.T) {
		for _, want := range config.HourFormats() {
			d.SetHours(want)

			if got := config.Get().Screen.Hours; got != want {
				t.Errorf("the file says %q, want %q", got, want)
			}
			if got := said(t, d.format); got != want.Label() {
				t.Errorf("the entity says %q, want %q", got, want.Label())
			}
		}
	})
}

func TestEverySwitchReachesBothSides(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	d := dash()

	for _, want := range []bool{true, false, true} {
		d.SetDate(want)

		if got := config.Get().Clock.Date; got != want {
			t.Errorf("the date in the file is %v, want %v", got, want)
		}
		if got := d.date.Get(); got != want {
			t.Errorf("the date entity is %v, want %v", got, want)
		}

	}
}

// The other direction: what Home Assistant sends lands in the file. The entity speaks labels and
// the setting speaks values, so this is also the decoder between them.
func TestEveryCommandFromHomeAssistantIsSaved(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	d := dash()

	for _, want := range config.Faces() {
		d.face.OnCommand(want.Label())

		if got := config.Get().Clock.Face; got != want {
			t.Errorf("%q from Home Assistant saved as %q", want, got)
		}
	}

	for _, want := range config.Inks() {
		d.ink.OnCommand(want.Label())

		if got := config.Get().Clock.Ink; got != want {
			t.Errorf("%q from Home Assistant saved as %q", want, got)
		}
	}

	d.date.OnCommand(false)
	if config.Get().Clock.Date {
		t.Error("turning the date off from Home Assistant did not save")
	}
}

// A label this build does not have changes nothing, rather than saving a value nothing can draw.
func TestAnUnknownLabelChangesNothing(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	d := dash()

	d.SetFace(config.FaceAnalog)
	d.face.OnCommand("Sundial")

	if got := config.Get().Clock.Face; got != config.FaceAnalog {
		t.Errorf("an unknown face left the setting on %q", got)
	}
}

// Restore is what runs when Home Assistant reconnects, and it has to put every entity back. One
// left out reads as its zero value on a device that has been set for months.
func TestRestorePutsEveryEntityBack(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	d := dash()

	cfg := config.Defaults()
	cfg.Clock.Face = config.FaceWords
	cfg.Clock.Position = config.PositionTop
	cfg.Clock.Size = config.SizeSmall
	cfg.Clock.Ink = config.InkAmber
	cfg.Clock.Date = false
	cfg.Screen.Hours = config.TwelveHour

	d.Restore(cfg)

	for _, at := range []struct {
		name string
		got  string
		want string
	}{
		{"face", said(t, d.face), cfg.Clock.Face.Label()},
		{"position", said(t, d.place), cfg.Clock.Position.Label()},
		{"size", said(t, d.size), cfg.Clock.Size.Label()},
		{"color", said(t, d.ink), cfg.Clock.Ink.Label()},
		{"hours", said(t, d.format), cfg.Screen.Hours.Label()},
	} {
		if at.got != at.want {
			t.Errorf("after restoring, %s says %q, want %q", at.name, at.got, at.want)
		}
	}

	if d.date.Get() != cfg.Clock.Date {
		t.Errorf("after restoring, the date is %v, want %v", d.date.Get(), cfg.Clock.Date)
	}
}

// Every entity the dashboard offers is one somebody can change. An entity with no command is a
// readout, and none of these are.
func TestEveryEntityTakesACommand(t *testing.T) {
	d := dash()

	for _, e := range d.Entities() {
		switch v := e.(type) {
		case *esphome.Select:
			if v.OnCommand == nil {
				t.Errorf("%s takes no command", v.ObjectID)
			}
		case *esphome.Switch:
			if v.OnCommand == nil {
				t.Errorf("%s takes no command", v.ObjectID)
			}
		default:
			t.Errorf("%T is neither a select nor a switch", e)
		}
	}
}
