package weather

import (
	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
)

const entityMost = 128

func init() {
	component.Register(component.Device, Get, component.Order(61))
}

type entities struct {
	entity    *esphome.Text
	look      *esphome.Select
	dashboard *esphome.Switch
	idle      *esphome.Switch
	animate   *esphome.Switch
	themed    *esphome.Switch
	position  *esphome.Select
	align     *esphome.Select
	size      *esphome.Select
}

func (w *Weather) Name() string { return "weather" }

func base(id, name, icon string) esphome.Base {
	return esphome.Base{
		ObjectID: id,
		Name:     name,
		Icon:     icon,
		Category: esphome.CategoryConfig,
		DeviceID: component.DeviceScreen,
	}
}

func (w *Weather) build() {
	w.ha = entities{
		entity: &esphome.Text{
			Base:      base("weather_entity", "Weather entity", "mdi:weather-partly-cloudy"),
			MaxLength: entityMost,
		},
		look: &esphome.Select{
			Base:    base("weather_look", "Weather look", "mdi:palette-outline"),
			Options: config.Labels(config.WeatherLooks()),
		},
		dashboard: &esphome.Switch{
			Base: base("weather_dashboard", "Weather on dashboard", "mdi:view-dashboard-outline"),
		},
		idle: &esphome.Switch{
			Base: base("weather_screensaver", "Weather on screen saver", "mdi:monitor-star"),
		},
		animate: &esphome.Switch{
			Base: base("weather_animate", "Animate weather", "mdi:animation-play-outline"),
		},
		themed: &esphome.Switch{
			Base: base("weather_themed", "Themed weather colors", "mdi:palette"),
		},
		position: &esphome.Select{
			Base:    base("weather_position", "Weather vertical position", "mdi:arrow-up-down"),
			Options: config.Labels(config.Positions()),
		},
		align: &esphome.Select{
			Base:    base("weather_align", "Weather horizontal position", "mdi:arrow-left-right"),
			Options: config.Labels(config.Aligns()),
		},
		size: &esphome.Select{
			Base:    base("weather_size", "Weather size", "mdi:resize"),
			Options: config.Labels(config.Sizes()),
		},
	}
	h := w.ha
	h.entity.OnCommand = w.SetEntity
	h.look.OnCommand = func(l string) {
		if v, ok := config.ByLabel(config.WeatherLooks(), l); ok {
			w.SetLook(v)
		}
	}
	h.dashboard.OnCommand = w.SetDashboard
	h.idle.OnCommand = w.SetIdle
	h.animate.OnCommand = w.SetAnimate
	h.themed.OnCommand = w.SetThemed
	h.position.OnCommand = func(l string) {
		if v, ok := config.ByLabel(config.Positions(), l); ok {
			w.SetPosition(v)
		}
	}
	h.align.OnCommand = func(l string) {
		if v, ok := config.ByLabel(config.Aligns(), l); ok {
			w.SetAlign(v)
		}
	}
	h.size.OnCommand = func(l string) {
		if v, ok := config.ByLabel(config.Sizes(), l); ok {
			w.SetSize(v)
		}
	}
}

func (w *Weather) Entities() []esphome.Entity {
	h := w.ha
	return []esphome.Entity{
		h.entity,
		h.look,
		h.dashboard,
		h.idle,
		h.animate,
		h.themed,
		h.position,
		h.align,
		h.size,
	}
}

func (w *Weather) Restore(cfg config.Config) { w.publish(cfg.Weather) }

func (w *Weather) publish(c config.Weather) {
	h := w.ha
	h.entity.Set(c.Entity)
	h.look.Set(c.Look.Label())
	h.dashboard.Set(c.Dashboard)
	h.idle.Set(c.Idle)
	h.animate.Set(c.Animate)
	h.themed.Set(c.Themed)
	h.position.Set(c.Position.Label())
	h.align.Set(c.Align.Label())
	h.size.Set(c.Size.Label())
}
