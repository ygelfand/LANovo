package screen

import (
	"path/filepath"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

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

func TestAnUnknownThemeKeepsWhatIsShowing(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	s := panel()

	s.Use(theme.DefaultName)
	s.Use("Chartreuse")

	if got := s.Theme().Name; got != theme.DefaultName {
		t.Errorf("an unknown theme left the screen on %q", got)
	}
}

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
		default:
			t.Errorf("%s is a %T, which this does not know how to check", e, e)
		}
	}
}
