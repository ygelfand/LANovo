package dashboard

import (
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"
	sharedlib "github.com/ygelfand/libcountertop/pkg/display/dashboard"

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
	return sharedlib.State{
		Hours:    cfg.Screen.Hours.Label(),
		Face:     cfg.Clock.Face.Label(),
		Position: cfg.Clock.Position.Label(),
		Size:     cfg.Clock.Size.Label(),
		Ink:      cfg.Clock.Ink.Label(),
		Date:     cfg.Clock.Date,
	}
}
func (d *Dashboard) Restore(cfg config.Config) { d.controls.Restore(state(cfg)) }

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
	d.controls = sharedlib.NewControls(
		sharedlib.Dependencies{
			Preferences: sharedlib.Preferences{
				Screen: config.ScreenSection,
				Clock:  config.ClockSection,
			},
			Display:  shell.Get(),
			DeviceID: component.DeviceScreen,
		},
	)
	d.format, d.face, d.place, d.size, d.ink = d.controls.Format, d.controls.Face, d.controls.Position, d.controls.Size, d.controls.Ink
	d.date = d.controls.Date
}
func (d *Dashboard) SetHours(v config.HourFormat)  { d.controls.SetHours(v) }
func (d *Dashboard) SetFace(v config.Face)         { d.controls.SetFace(v) }
func (d *Dashboard) SetPosition(v config.Position) { d.controls.SetPosition(v) }
func (d *Dashboard) SetSize(v config.Size)         { d.controls.SetSize(v) }
func (d *Dashboard) SetInk(v config.Ink)           { d.controls.SetInk(v) }
func (d *Dashboard) SetDate(v bool)                { d.controls.SetDate(v) }
