package screen

import (
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/display/style"
	"github.com/ygelfand/libcountertop/pkg/hook"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/setting"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() {
	component.Register(component.Device, Get, component.Order(10))
}

type Screen struct {
	Themed hook.Hook[theme.Theme]

	Restyled hook.Hook[config.Screen]

	knobs *setting.Controls[config.Screen]
	level *esphome.Sensor

	mu      sync.Mutex
	current theme.Theme

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

func (s *Screen) knob(name string) esphome.Entity { return s.knobs.Entity(name) }

func (s *Screen) Restore(c config.Config) {
	if c.Screen.Mode != config.ModeAuto {
		s.applyBacklight(c.Screen.Backlight)
	}

	s.Use(c.Screen.Theme)
	s.knobs.PublishFrom(c.Screen)
}

func (s *Screen) Theme() theme.Theme {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

func (s *Screen) Use(name string) {
	t, ok := theme.ByName(style.Theme(config.Get().Screen.Style, name))
	if !ok {
		slog.Warn("no such theme", "theme", name)
		return
	}

	s.mu.Lock()
	changed := s.current.Name != t.Name
	s.current = t
	s.mu.Unlock()

	at := config.Get().Screen
	at.Theme = t.Name
	s.knobs.PublishFrom(at)

	if changed {
		s.Themed.Emit(t)
	}
}

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

func (s *Screen) Lit(percent int) {
	s.mu.Lock()
	same := s.lit == percent
	s.lit = percent
	s.mu.Unlock()

	if !same {
		s.level.Set(float32(percent))
	}
}

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

func (s *Screen) SetMode(mode config.ScreenMode) {
	if err := config.Set().Screen().Mode(mode); err != nil {
		slog.Error("saving a setting failed", "setting", "auto", "err", err)
	}

	if mode == config.ModeManual {
		s.applyBacklight(config.Get().Screen.Backlight)
	}
	s.knobs.Publish()
}
