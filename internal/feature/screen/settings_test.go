package screen

import (
	"path/filepath"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// The same property the dashboard's settings are held to: a setter reaches the file and the entity
// both, and a command from Home Assistant reaches the file. This is the drift that has already
// happened three times, the dock edge among them.
//
// Built directly rather than through Get, which also registers a settings page. The entities are
// all that is under test.
func panel() *Screen {
	s := &Screen{lit: -1}
	s.build()
	return s
}

func TestSettingTheDrawerEdgeTellsBothSides(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	s := panel()

	for _, want := range config.Edges() {
		s.SetDrawer(want)

		if got := config.Get().Screen.Drawer; got != want {
			t.Errorf("the file says %q, want %q", got, want)
		}
		if got := s.knob("drawer").(*esphome.Select).Get(); got != want.Label() {
			t.Errorf("the entity says %q, want %q", got, want.Label())
		}
	}
}

func TestTheDrawerCommandFromHomeAssistantIsSaved(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	s := panel()

	for _, want := range config.Edges() {
		s.knob("drawer").(*esphome.Select).OnCommand(want.Label())

		if got := config.Get().Screen.Drawer; got != want {
			t.Errorf("%q from Home Assistant saved as %q", want, got)
		}
	}
}

func TestSettingTheModeTellsBothSides(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	s := panel()

	// Automatic last, so the manual case does not go on to touch the panel this test has no way to
	// reach. Manual only saves and reports; applying the level is the device's business.
	for _, want := range []config.ScreenMode{config.ModeManual, config.ModeAuto} {
		s.SetMode(want)

		if got := config.Get().Screen.Mode; got != want {
			t.Errorf("the file says %q, want %q", got, want)
		}
		if got := s.knob("auto").(*esphome.Switch).Get(); got != (want == config.ModeAuto) {
			t.Errorf("the entity says %v for %q", got, want)
		}
	}
}

// The theme is the one that was already right, and worth holding so it stays that way: Use sets the
// entity, so the settings row writing the file and calling Use covers both sides between them.
func TestUsingAThemeTellsBothSides(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	s := panel()

	for _, want := range theme.All {
		s.Use(want.Name)

		if got := s.knob("theme").(*esphome.Select).Get(); got != want.Name {
			t.Errorf("the entity says %q, want %q", got, want.Name)
		}
		if got := s.Theme().Name; got != want.Name {
			t.Errorf("the screen is drawing %q, want %q", got, want.Name)
		}
	}
}

// A theme this build does not have leaves the one on screen alone, rather than blanking it.
func TestAnUnknownThemeKeepsWhatIsShowing(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	s := panel()

	s.Use(theme.DefaultName)
	s.Use("Chartreuse")

	if got := s.Theme().Name; got != theme.DefaultName {
		t.Errorf("an unknown theme left the screen on %q", got)
	}
}

// On automatic the slider is a bias rather than a level, so setting it reports and saves without
// touching the panel. That is the half of the backlight reachable without one.
func TestTheBacklightBiasTellsBothSidesOnAutomatic(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	s := panel()

	s.SetMode(config.ModeAuto)

	for _, want := range []int{0, 37, 76, 100} {
		s.SetBacklight(want)

		if got := config.Get().Screen.Backlight; got != want {
			t.Errorf("the file says %d, want %d", got, want)
		}
		if got := s.knob("backlight").(*esphome.Number).Get(); got != float32(want) {
			t.Errorf("the entity says %v, want %d", got, want)
		}
	}
}

// Out of range is brought into it rather than saved as given: the slider is a percentage and a
// device that saved 150 would report a level it can never be at.
func TestTheBacklightIsHeldToItsRange(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	s := panel()

	s.SetMode(config.ModeAuto)

	s.SetBacklight(150)
	if got := config.Get().Screen.Backlight; got != 100 {
		t.Errorf("150 saved as %d, want 100", got)
	}

	s.SetBacklight(-20)
	if got := config.Get().Screen.Backlight; got != 0 {
		t.Errorf("-20 saved as %d, want 0", got)
	}
}

// Every entity the screen offers is either something to change or something to read, and the ones
// that take a command have to have one.
func TestEverySettableEntityTakesACommand(t *testing.T) {
	s := panel()

	for _, e := range s.Entities() {
		switch v := e.(type) {
		case *esphome.Select:
			if v.OnCommand == nil {
				t.Errorf("%s takes no command", v.ObjectID)
			}
		case *esphome.Switch:
			if v.OnCommand == nil {
				t.Errorf("%s takes no command", v.ObjectID)
			}
		case *esphome.Number:
			if v.OnCommand == nil {
				t.Errorf("%s takes no command", v.ObjectID)
			}
		case *esphome.Sensor:
			// A readout. Nothing to command.
		default:
			t.Errorf("%s is a %T, which this does not know how to check", e, e)
		}
	}
}
