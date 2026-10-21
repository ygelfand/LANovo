package frontend

import (
	"image"
	"net/http"
	"sync"

	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/screencmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/webapi"
	"github.com/ygelfand/libcountertop/pkg/runtime/webui"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/brand"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/alarm"
	"github.com/ygelfand/LANovo/internal/feature/alerts"
	"github.com/ygelfand/LANovo/internal/feature/api"
	"github.com/ygelfand/LANovo/internal/feature/broadcast"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/layout"
)

type componentFrontend struct{ *webapi.API }

func (c componentFrontend) Restore(cfg config.Config) { c.API.Restore(cfg.Frontend) }

var get = sync.OnceValue(func() *webapi.API {
	a := webapi.New(webapi.Options{
		Settings: config.FrontendSection,
		Registry: &component.Settings,
		States:   api.Get(),
		Device:   device,
		DeviceID: component.DeviceFrontend,
		Logo:     webapi.Logo{Light: brand.LogoLight, Dark: brand.LogoDark},
		Pages: map[string]http.Handler{
			"alarms":    alarm.Get().Handler(alerts.Get().Tones),
			"call":      call.Get().Handler(discovery.Get().Peers),
			"broadcast": broadcast.Get().WebHandler(),
		},
		LogTag: layout.LogTag,
		Screen: func() (image.Image, error) { return screencmd.Composed(display.Get()) },
	})
	component.Settings.Add(a.Controls())
	return a
})

func Get() *webapi.API { return get() }

var handler = sync.OnceValue(func() http.Handler { return Get().Handler() })

func init() {
	component.Register(
		sharedcomponent.Network,
		func() componentFrontend { return componentFrontend{Get()} },
	)
	web.Get().HandlePublic(
		webapi.Prefix+"/*",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler().ServeHTTP(w, r)
		}),
	)
	web.Get().PublicRoot(webui.Handler())
}

func device() webapi.Device {
	b := board.Current()
	return webapi.Device{
		ID:      discovery.ID(),
		Name:    config.Get().Device.Name,
		Model:   b.Model,
		Board:   b.Name,
		Version: layout.Version,
	}
}
