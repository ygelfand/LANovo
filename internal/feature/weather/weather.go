package weather

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/weather"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/product"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(61))
}

type Weather struct{ *weather.Weather }

func (w Weather) Restore(cfg config.Config) { w.Publish(cfg.Weather) }

var get = sync.OnceValue(func() *Weather {
	return &Weather{weather.Build(product.Get(), config.WeatherSection, config.HomeSection)}
})

func Get() *Weather { return get() }
