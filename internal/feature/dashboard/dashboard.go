// Package dashboard is what the screen shows when nothing else is asking.
//
// It holds the lowest claim, so anything with something to say covers it. The clock face is what
// it draws today and is not the only thing it will draw.
package dashboard

import (
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	sharedlib "github.com/ygelfand/libcountertop/pkg/display/dashboard"
)

func init() {
	component.Register(component.Device, Get, component.Order(30))
}

type Dashboard struct {
	controls *sharedlib.Controls
	format   *esphome.Select
	face     *esphome.Select
	place    *esphome.Select
	size     *esphome.Select
	ink      *esphome.Select
	date     *esphome.Switch
}

var (
	once   sync.Once
	shared *Dashboard
)

func Get() *Dashboard {
	once.Do(func() {
		shared = &Dashboard{}
		shared.build()

		// Neither is the clock's own doing, and both change what it should look like.
		screen.Get().Themed.Listen(func(theme.Theme) { shared.Redraw() })
		sensors.Get().Turned.Listen(func(display.Orientation) { shared.Redraw() })
		poster.Get().Changed.Listen(func(int) { shared.Redraw() })
		shell.Get().Changed.Listen(func(c shell.Change) {
			if _, ok := c.To.(*idle.View); ok {
				Clock()
			}
		})
	})
	return shared
}

func (d *Dashboard) Name() string { return "dashboard" }

func (d *Dashboard) Entities() []esphome.Entity { return d.controls.Entities() }
func state(cfg config.Config) sharedlib.State {
	return sharedlib.State{Hours: cfg.Screen.Hours.Label(), Face: cfg.Clock.Face.Label(), Position: cfg.Clock.Position.Label(), Size: cfg.Clock.Size.Label(), Ink: cfg.Clock.Ink.Label(), Date: cfg.Clock.Date}
}
func (d *Dashboard) Restore(cfg config.Config) { d.controls.Restore(state(cfg)) }

// Ready reports whether the clock has anything worth showing, which is the time being right. The
// boot screen holds until it does: this device starts in 1970, and a confident wrong time is
// worse than a logo.
func (d *Dashboard) Ready() bool {
	select {
	case <-clock.Get().Ready():
		return true
	default:
		return false
	}
}

func (d *Dashboard) Redraw() { shell.Get().Redraw() }

func (d *Dashboard) build() {
	d.controls = sharedlib.NewControls(sharedlib.ControlOptions{
		DeviceID: component.DeviceScreen, Read: func() sharedlib.State { return state(config.Get()) }, Redraw: d.Redraw,
		Hours:    sharedlib.Choices(config.HourFormats(), func(v config.HourFormat) error { return config.Set().Screen().Hours(v) }),
		Face:     sharedlib.Choices(config.Faces(), func(v config.Face) error { return config.Set().Clock().Face(v) }),
		Position: sharedlib.Choices(config.Positions(), func(v config.Position) error { return config.Set().Clock().Position(v) }),
		Size:     sharedlib.Choices(config.Sizes(), func(v config.Size) error { return config.Set().Clock().Size(v) }),
		Ink:      sharedlib.Choices(config.Inks(), func(v config.Ink) error { return config.Set().Clock().Ink(v) }),
		Date:     func(v bool) error { return config.Set().Clock().Date(v) },
	})
	d.format, d.face, d.place, d.size, d.ink = d.controls.Format, d.controls.Face, d.controls.Position, d.controls.Size, d.controls.Ink
	d.date = d.controls.Date
}
func (d *Dashboard) SetHours(v config.HourFormat) {
	d.controls.Commit("clock_format", func() error { return config.Set().Screen().Hours(v) })
}
func (d *Dashboard) SetFace(v config.Face) {
	d.controls.Commit("clock_face", func() error { return config.Set().Clock().Face(v) })
}
func (d *Dashboard) SetPosition(v config.Position) {
	d.controls.Commit("clock_position", func() error { return config.Set().Clock().Position(v) })
}
func (d *Dashboard) SetSize(v config.Size) {
	d.controls.Commit("clock_size", func() error { return config.Set().Clock().Size(v) })
}
func (d *Dashboard) SetInk(v config.Ink) {
	d.controls.Commit("clock_color", func() error { return config.Set().Clock().Ink(v) })
}
func (d *Dashboard) SetDate(v bool) {
	d.controls.Commit("clock_date", func() error { return config.Set().Clock().Date(v) })
}
