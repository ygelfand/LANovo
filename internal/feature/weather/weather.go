package weather

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/weather"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/states"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(61))
}

type Weather struct{ *weather.Weather }

func (w Weather) Restore(cfg config.Config) { w.Publish(cfg.Weather) }

var get = sync.OnceValue(func() *Weather {
	w := &Weather{weather.New(weather.Options{
		Controller: homeassistant.Get(),
		States:     states.Get(),
		Settings:   config.WeatherSection,
		Home:       config.HomeSection,
		Redraw:     func() { shell.Get().Redraw() },
		DeviceID:   component.DeviceScreen,
	})}
	component.Settings.Add(w.Settings())
	return w
})

func Get() *Weather { return get() }
