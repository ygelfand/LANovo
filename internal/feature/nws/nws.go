package nws

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/nws"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/remember"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/product"
	featureremember "github.com/ygelfand/LANovo/internal/feature/remember"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(62))
}

var get = sync.OnceValue(func() *nws.NWS {
	return nws.Build(
		product.Get(),
		config.WeatherSection,
		remember.NewSlot[[]string](featureremember.Get(), "weather.dismissed"),
	)
})

func Get() *nws.NWS { return get() }
