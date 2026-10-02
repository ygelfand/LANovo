// Package dashboard is what the screen shows when nothing else is asking.
//
// It holds the lowest claim, so anything with something to say covers it. The clock face is what
// it draws today and is not the only thing it will draw.
package dashboard

import (
	"log/slog"
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
)

func init() {
	component.Register(component.Device, Get, component.Order(30))
}

type Dashboard struct {
	format *esphome.Select
	face   *esphome.Select
	place  *esphome.Select
	size   *esphome.Select
	ink    *esphome.Select
	date   *esphome.Switch
	logo   *esphome.Switch
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

func (d *Dashboard) Entities() []esphome.Entity {
	return []esphome.Entity{
		d.format, d.face,
		d.place, d.size, d.ink, d.date, d.logo,
	}
}

func (d *Dashboard) Restore(cfg config.Config) {
	d.format.Set(cfg.Screen.Hours.Label())
	d.face.Set(cfg.Clock.Face.Label())
	d.place.Set(cfg.Clock.Position.Label())
	d.size.Set(cfg.Clock.Size.Label())
	d.ink.Set(cfg.Clock.Ink.Label())
	d.date.Set(cfg.Clock.Date)
	d.logo.Set(cfg.Screen.Logo)
}

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
	d.format = &esphome.Select{
		Base: esphome.Base{
			ObjectID: "clock_format",
			Name:     "Clock format",
			Icon:     "mdi:clock-outline",
			Category: esphome.CategoryConfig,
			DeviceID: component.DeviceScreen,
		},
		Options: config.Labels(config.HourFormats()),
	}

	d.format.OnCommand = func(label string) {
		if hours, ok := config.ByLabel(config.HourFormats(), label); ok {
			d.SetHours(hours)
		}
	}

	d.face = &esphome.Select{
		Base: esphome.Base{
			ObjectID: "clock_face",
			Name:     "Clock face",
			Icon:     "mdi:clock-digital",
			Category: esphome.CategoryConfig,
			DeviceID: component.DeviceScreen,
		},
		Options: config.Labels(config.Faces()),
	}

	d.face.OnCommand = func(label string) {
		if kind, ok := config.ByLabel(config.Faces(), label); ok {
			d.SetFace(kind)
		}
	}

	d.place = &esphome.Select{
		Base: esphome.Base{
			ObjectID: "clock_position",
			Name:     "Clock position",
			Icon:     "mdi:align-vertical-center",
			Category: esphome.CategoryConfig,
			DeviceID: component.DeviceScreen,
		},
		Options: config.Labels(config.Positions()),
	}

	d.place.OnCommand = func(label string) {
		if at, ok := config.ByLabel(config.Positions(), label); ok {
			d.SetPosition(at)
		}
	}

	d.size = &esphome.Select{
		Base: esphome.Base{
			ObjectID: "clock_size",
			Name:     "Clock size",
			Icon:     "mdi:format-size",
			Category: esphome.CategoryConfig,
			DeviceID: component.DeviceScreen,
		},
		Options: config.Labels(config.Sizes()),
	}

	d.size.OnCommand = func(label string) {
		if size, ok := config.ByLabel(config.Sizes(), label); ok {
			d.SetSize(size)
		}
	}

	d.ink = &esphome.Select{
		Base: esphome.Base{
			ObjectID: "clock_color",
			Name:     "Clock color",
			Icon:     "mdi:palette-outline",
			Category: esphome.CategoryConfig,
			DeviceID: component.DeviceScreen,
		},
		Options: config.Labels(config.Inks()),
	}

	d.ink.OnCommand = func(label string) {
		if ink, ok := config.ByLabel(config.Inks(), label); ok {
			d.SetInk(ink)
		}
	}

	d.date = &esphome.Switch{
		Base: esphome.Base{
			ObjectID: "clock_date",
			Name:     "Clock date",
			Icon:     "mdi:calendar-blank-outline",
			Category: esphome.CategoryConfig,
			DeviceID: component.DeviceScreen,
		},
	}

	d.date.OnCommand = d.SetDate

	d.logo = &esphome.Switch{
		Base: esphome.Base{
			ObjectID: "dashboard_logo",
			Name:     "Dashboard logo",
			Icon:     "mdi:home-assistant",
			Category: esphome.CategoryConfig,
			DeviceID: component.DeviceScreen,
		},
	}

	d.logo.OnCommand = d.SetLogo
}

// SetLogo shows or hides the mark and remembers it.
func (d *Dashboard) SetLogo(on bool) {
	d.logo.Set(on)

	if err := config.Set().Screen().Logo(on); err != nil {
		slog.Error("saving a setting failed", "setting", d.logo.ObjectID, "err", err)
		return
	}
	d.Redraw()
}

// SetPosition moves the clock up or down the glass and remembers it.
func (d *Dashboard) SetPosition(at config.Position) {
	d.place.Set(at.Label())

	if err := config.Set().Clock().Position(at); err != nil {
		slog.Error("saving a setting failed", "setting", d.place.ObjectID, "err", err)
		return
	}
	d.Redraw()
}

// SetSize changes how much of its room the clock fills and remembers it.
func (d *Dashboard) SetSize(size config.Size) {
	d.size.Set(size.Label())

	if err := config.Set().Clock().Size(size); err != nil {
		slog.Error("saving a setting failed", "setting", d.size.ObjectID, "err", err)
		return
	}
	d.Redraw()
}

// SetInk changes the color the clock is drawn in and remembers it.
func (d *Dashboard) SetInk(ink config.Ink) {
	d.ink.Set(ink.Label())

	if err := config.Set().Clock().Ink(ink); err != nil {
		slog.Error("saving a setting failed", "setting", d.ink.ObjectID, "err", err)
		return
	}
	d.Redraw()
}

// SetDate shows or hides the day under the time and remembers it.
func (d *Dashboard) SetDate(on bool) {
	d.date.Set(on)

	if err := config.Set().Clock().Date(on); err != nil {
		slog.Error("saving a setting failed", "setting", d.date.ObjectID, "err", err)
		return
	}
	d.Redraw()
}

// SetFace changes how the clock is drawn and remembers it.
func (d *Dashboard) SetFace(kind config.Face) {
	d.face.Set(kind.Label())

	if err := config.Set().Clock().Face(kind); err != nil {
		slog.Error("saving a setting failed", "setting", d.face.ObjectID, "err", err)
		return
	}
	d.Redraw()
}

// SetHours changes the clock format and remembers it. Everything that changes the format comes
// through here, so Home Assistant is told whatever asked for it.
func (d *Dashboard) SetHours(hours config.HourFormat) {
	d.format.Set(hours.Label())

	if err := config.Set().Screen().Hours(hours); err != nil {
		slog.Error("saving a setting failed", "setting", d.format.ObjectID, "err", err)
		return
	}
	d.Redraw()
}
