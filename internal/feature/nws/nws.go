package nws

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/nws"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/remember"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	featureremember "github.com/ygelfand/LANovo/internal/feature/remember"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(62))
}

var get = sync.OnceValue(func() *nws.NWS {
	return nws.New(nws.Options{
		Controller: homeassistant.Get(),
		Agent:      "(LANovo " + layout.Version + ", github.com/ygelfand/LANovo)",
		Settings:   config.WeatherSection,
		Dismissed:  remember.NewSlot[[]string](featureremember.Get(), "weather.dismissed"),
		Redraw:     func() { shell.Get().Redraw() },
	})
})

func Get() *nws.NWS { return get() }
