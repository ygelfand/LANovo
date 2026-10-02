package screen

import (
	"path/filepath"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func fresh(t *testing.T) *Screen {
	t.Helper()

	config.Use(filepath.Join(t.TempDir(), "state.json"))
	return Get()
}

func TestRestoreTakesTheSavedTheme(t *testing.T) {
	s := fresh(t)

	cfg := config.Defaults()
	cfg.Screen.Theme = "Paper"
	s.Restore(cfg)

	if got := s.Theme().Name; got != "Paper" {
		t.Errorf("theme = %q, want Paper", got)
	}
}

// A theme name that is no longer one we have must leave the screen as it is rather than blanking
// it or falling back to something the user did not choose.
func TestUnknownThemeIsKept(t *testing.T) {
	s := fresh(t)

	cfg := config.Defaults()
	cfg.Screen.Theme = "Midnight"
	s.Restore(cfg)

	cfg.Screen.Theme = "Nonesuch"
	s.Restore(cfg)

	if got := s.Theme().Name; got != "Midnight" {
		t.Errorf("theme = %q, want it held at Midnight", got)
	}
}

func TestThemedFiresOnChange(t *testing.T) {
	s := fresh(t)
	s.Use("Midnight")

	got := make(chan theme.Theme, 4)
	cancel := s.Themed.Listen(func(t theme.Theme) { got <- t })
	defer cancel()

	s.Use("Ocean")

	select {
	case th := <-got:
		if th.Name != "Ocean" {
			t.Errorf("Themed carried %q, want Ocean", th.Name)
		}
	default:
		t.Fatal("Themed did not fire")
	}
}

// Whatever is drawing repaints on the hook, so firing for a theme that did not change would
// repaint the screen for nothing.
func TestThemedIsQuietWhenNothingChanged(t *testing.T) {
	s := fresh(t)
	s.Use("Midnight")

	got := make(chan theme.Theme, 4)
	cancel := s.Themed.Listen(func(t theme.Theme) { got <- t })
	defer cancel()

	s.Use("Midnight")

	select {
	case th := <-got:
		t.Errorf("Themed fired for an unchanged theme: %q", th.Name)
	default:
	}
}

func TestEntities(t *testing.T) {
	s := fresh(t)

	if got := len(s.Entities()); got != 7 {
		t.Errorf("%d entities, want backlight, mode, theme, style, size, the drawer edge and the level", got)
	}
	for i, e := range s.Entities() {
		if e == nil {
			t.Errorf("entity %d is nil", i)
		}
	}
}

// On automatic the slider is a bias, so setting it must not drive the panel: the next reading is
// a quarter second away and would take it straight back off whatever was set here.
func TestSettingTheSliderOnAutomaticOnlyRemembersIt(t *testing.T) {
	s := fresh(t)

	if err := config.Set().Screen().Mode(config.ModeAuto); err != nil {
		t.Fatalf("setting automatic: %v", err)
	}
	s.Lit(64)

	s.SetBacklight(20)

	if got := config.Get().Screen.Backlight; got != 20 {
		t.Errorf("the slider stored %d, want 20", got)
	}
	if got := s.knob("backlight").(*esphome.Number).Get(); got != 20 {
		t.Errorf("the entity reads %v, want 20", got)
	}
	if got := s.level.Get(); got != 64 {
		t.Errorf("the panel was driven to %v by the slider, want it left at 64", got)
	}
}

// On manual it is a level, and does.
func TestSettingTheSliderOnManualDrivesThePanel(t *testing.T) {
	s := fresh(t)

	if err := config.Set().Screen().Mode(config.ModeManual); err != nil {
		t.Fatalf("setting manual: %v", err)
	}

	s.SetBacklight(20)

	if got := config.Get().Screen.Backlight; got != 20 {
		t.Errorf("the slider stored %d, want 20", got)
	}
	if got := s.level.Get(); got != 20 {
		t.Errorf("the panel reads %v, want the 20 that was set", got)
	}
}

// Every edge has to be selectable, or a device mounted so the rail is behind something cannot be
// fixed from Home Assistant.
func TestEveryEdgeIsOffered(t *testing.T) {
	s := fresh(t)

	for _, e := range config.Edges() {
		var found bool
		for _, o := range s.knob("drawer").(*esphome.Select).Options {
			if o == e.Label() {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the %q edge is not offered", e.Label())
		}
	}
}

func TestTheDrawerEdgeIsRestored(t *testing.T) {
	s := fresh(t)

	cfg := config.Defaults()
	cfg.Screen.Drawer = config.EdgeTop
	s.Restore(cfg)

	if got := s.knob("drawer").(*esphome.Select).Get(); got != config.EdgeTop.Label() {
		t.Errorf("the drawer select reads %q, want %q", got, config.EdgeTop.Label())
	}
}

// Every theme has to be selectable, or one of them cannot be reached from Home Assistant.
func TestEveryThemeIsOffered(t *testing.T) {
	s := fresh(t)

	offered := map[string]bool{}
	for _, name := range s.knob("theme").(*esphome.Select).Options {
		offered[name] = true
	}

	for _, th := range theme.All {
		if !offered[th.Name] {
			t.Errorf("%s is not offered as a choice", th.Name)
		}
	}
}
