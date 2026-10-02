// Package screen is the panel as Home Assistant sees it: how bright it is, and how that is
// decided.
package screen

import (
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/lib/hook"
	"github.com/ygelfand/LANovo/internal/setting"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() {
	component.Register(component.Device, Get, component.Order(10))
}

type Screen struct {
	// Themed carries every change of palette, for whatever is drawing to paint again.
	Themed hook.Hook[theme.Theme]

	Restyled hook.Hook[config.Screen]

	// knobs renders the table as entities. level is not one of them: it reports what the panel
	// came to and nothing sets it.
	knobs *setting.Controls[config.Screen]
	level *esphome.Sensor

	mu      sync.Mutex
	current theme.Theme

	// lit is the last level reported, so a reading four times a second that works out the same
	// is not four messages a second. Below any real level, so the first one is news.
	lit int
}

var (
	once   sync.Once
	shared *Screen
)

func Get() *Screen {
	once.Do(func() {
		shared = &Screen{lit: -1}
		shared.build()
	})
	return shared
}

func (s *Screen) Name() string { return "screen" }

func (s *Screen) Entities() []esphome.Entity {
	return append(s.knobs.Entities(), s.level)
}

// knob is what one row was rendered as, for reading back what Home Assistant was told.
func (s *Screen) knob(name string) esphome.Entity { return s.knobs.Entity(name) }

// Restore puts the panel back where it was left, and the entities with it.
//
// On automatic the stored level is the bias, not a level, so applying it would light the panel at
// whatever the slider says and then correct it on the first reading a moment later. The panel
// keeps what the boot screen left it at until the room is read.
func (s *Screen) Restore(c config.Config) {
	if c.Screen.Mode != config.ModeAuto {
		s.applyBacklight(c.Screen.Backlight)
	}

	s.Use(c.Screen.Theme)
	s.knobs.PublishFrom(c.Screen)
}

// Theme is the palette everything on screen draws with.
func (s *Screen) Theme() theme.Theme {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// Use takes a theme by name, keeping what is shown when the name is not one we have.
func (s *Screen) Use(name string) {
	t, ok := theme.ByName(name)
	if !ok {
		slog.Warn("no such theme", "theme", name)
		return
	}

	s.mu.Lock()
	changed := s.current.Name != t.Name
	s.current = t
	s.mu.Unlock()

	// The palette is taken without being written — Restore hands one that is already stored, and a
	// row that picked one wrote it first — so the entity is told what is being drawn, not what the
	// file happens to say.
	at := config.Get().Screen
	at.Theme = t.Name
	s.knobs.PublishFrom(at)

	if changed {
		s.Themed.Emit(t)
	}
}

// SetBacklight applies a level and remembers it. Everything that changes the backlight comes
// through here, so Home Assistant is told whatever asked for it.
//
// On automatic it is not applied. The level is the bias the light sensor reads its curve through,
// and the next reading is a quarter of a second away, so setting the panel here would put it
// somewhere the room does not call for and then take it away again.
func (s *Screen) SetBacklight(level int) {
	settled := max(0, min(level, 100))
	if config.Get().Screen.Mode != config.ModeAuto {
		settled = s.applyBacklight(settled)
	}

	if err := config.Set().Screen().Backlight(settled); err != nil {
		slog.Error("saving a setting failed", "setting", "backlight", "err", err)
	}
	s.knobs.Publish()
}

// Lit says what the panel is actually at, which on automatic is not what the slider says. Nothing
// else reports it, and with the slider turned into a bias there would otherwise be no way to see
// what the room has come to.
func (s *Screen) Lit(percent int) {
	s.mu.Lock()
	same := s.lit == percent
	s.lit = percent
	s.mu.Unlock()

	if !same {
		s.level.Set(float32(percent))
	}
}

// applyBacklight drives the panel and reports the level it settled on, which is what is stored and
// published. A panel that will not take it keeps the level it had.
func (s *Screen) applyBacklight(level int) int {
	level = max(0, min(level, 100))

	if err := display.Get().Brightness(level); err != nil {
		slog.Error("setting the backlight failed", "level", level, "err", err)
		return config.Get().Screen.Backlight
	}

	s.Lit(level)
	return level
}

func (s *Screen) build() {
	s.current = theme.Default()

	s.knobs = &setting.Controls[config.Screen]{
		Table:  Table(),
		Device: component.DeviceScreen,
		Read:   func() config.Screen { return config.Get().Screen },
		Save:   s.save,
	}

	s.level = &esphome.Sensor{
		Base: esphome.Base{
			ObjectID: "screen_brightness",
			DeviceID: component.DeviceScreen,
			Name:     "Screen brightness",
			Icon:     "mdi:brightness-percent",
			Category: esphome.CategoryDiagnostic,
		},
		Unit:       "%",
		StateClass: esphome.StateClassMeasurement,
	}
}

// save is what a row does when Home Assistant moves it: the row writes the field, the store takes
// the whole thing, and the panel is driven to match.
//
// Unlike the camera's, these settings are the stored form, so there is no separate apply step to
// carry a change onto hardware later — the backlight and the palette are put where the settings
// now say, here.
func (s *Screen) save(row Setting, value string) error {
	now := config.Get().Screen
	if err := row.Write(&now, value); err != nil {
		return err
	}
	if err := config.Set().Screen().All(now); err != nil {
		return err
	}

	if now.Mode == config.ModeManual {
		s.applyBacklight(now.Backlight)
	}
	s.Use(now.Theme)
	s.Restyled.Emit(now)
	return nil
}

// SetDrawer moves the edge the dock comes in from and remembers it. Everything that changes it
// comes through here, so Home Assistant is told whatever asked for it.
func (s *Screen) Set(name, value string) error {
	if err := s.save(Table().Row(name), value); err != nil {
		return err
	}
	s.knobs.Publish()
	return nil
}

func (s *Screen) SetDrawer(edge config.Edge) {
	if err := config.Set().Screen().Drawer(edge); err != nil {
		slog.Error("saving a setting failed", "setting", "drawer", "err", err)
	}
	s.knobs.Publish()
}

// SetMode turns automatic brightness on or off and remembers it. Everything that changes the mode
// comes through here, so Home Assistant is told whatever asked for it.
func (s *Screen) SetMode(mode config.ScreenMode) {
	if err := config.Set().Screen().Mode(mode); err != nil {
		slog.Error("saving a setting failed", "setting", "auto", "err", err)
	}

	// Auto drives the panel without touching the stored level, so going back to manual has to put
	// it where the slider says rather than leave it at the room's brightness.
	if mode == config.ModeManual {
		s.applyBacklight(config.Get().Screen.Backlight)
	}
	s.knobs.Publish()
}
