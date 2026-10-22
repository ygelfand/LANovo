package tessera

import (
	"path/filepath"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"
	"github.com/ygelfand/libcountertop/pkg/display/theme"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"github.com/ygelfand/libcountertop/pkg/settings/timezone"
	shared "github.com/ygelfand/libcountertop/pkg/tessera"
	"github.com/ygelfand/libcountertop/pkg/tessera/firmwarerelay"
	"github.com/ygelfand/libcountertop/pkg/tessera/firmwareview"
	tesseraview "github.com/ygelfand/libcountertop/pkg/tessera/widgets"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/states"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() {
	component.Register(sharedcomponent.Network, Get, sharedcomponent.Order(95))
	component.Register(sharedcomponent.Network, Firmware, sharedcomponent.Order(96))
}

var get = sync.OnceValue(func() *shared.Set {
	home := homecontrol.Get()
	alert, unalert := tesseraview.Alerter(message.Get())
	front := func() {
		shell.Get().Close()
		idle.Get().Wake()
		dashboard.Tabs().Clock()
	}
	screen := func(f shared.Facing, device uint32, file string, on func() bool) *shared.Screen {
		return shared.New(shared.Options{
			Facing:   f,
			Device:   device,
			Board:    func() string { return board.Current().Tessera.Board },
			Size:     display.Get().Size,
			Grid:     home.Grid,
			Node:     func() string { return layout.Slug(config.Get().Device.Name) },
			Name:     func() string { return config.Get().Device.Name },
			Speaking: func() bool { return on() && !home.Firmware() },
			Keep:     filepath.Join(layout.TesseraDir, file),
			Alert:    alert,
			Unalert:  unalert,
			Front:    front,
		})
	}
	var set *shared.Set
	if board.Current().Motion {
		set = shared.NewSet(
			screen(
				shared.Landscape,
				component.DeviceLandscape,
				"tessera-landscape.json",
				home.Landscape,
			),
			screen(
				shared.Portrait,
				component.DevicePortrait,
				"tessera-portrait.json",
				home.Portrait,
			),
		)
	} else {
		set = shared.NewSet(
			screen(
				shared.Dashboard,
				component.DeviceDashboard,
				"tessera-dashboard.json",
				home.Tessera,
			),
		)
	}
	home.Speaking.Listen(func(schema.HomeMode) { component.Reconnect.Emit(struct{}{}) })
	home.Regrid.Listen(func(schema.TesseraGrid) { set.Regrid() })
	sensors.Get().Turned.Listen(func(geometry.Orientation) { set.Regrid() })
	display.Get().Attached.Listen(func(struct{}) { set.Regrid() })
	set.Changed.Listen(func(struct{}) { shell.Get().Redraw() })
	set.Told.Listen(component.Fire.Emit)
	firmware()
	return set
})

func Get() *shared.Set { return get() }

var firmware = sync.OnceValue(func() *firmwareview.View {
	v := firmwareview.New(firmwareview.Options{
		Helper:   layout.TesseraHelper,
		Screen:   screens(),
		Panel:    display.Get().Size,
		Diagonal: board.Current().Diagonal,
		Language: func() string { return config.Get().Screen.Language },
		Grid: func() (int, int) {
			g := homecontrol.Get().Grid()
			return g.Columns, g.Rows
		},
		Theme:   func() theme.Theme { return widgets.Palette(config.Get().Screen) },
		Themed:  homecontrol.Get().Themed,
		Turn:    homecontrol.Get().Turn,
		Bottom:  homecontrol.Get().Bottom,
		Follow:  homecontrol.Get().Follow,
		Product: layout.Manufacturer,
		Board:   board.Current().Tessera.Board,
		Redraw:  shell.Get().Redraw,
	})
	home := homecontrol.Get()
	home.Redrawn.Listen(func(on bool) {
		v.Set(on)
		component.Reconnect.Emit(struct{}{})
	})
	display.Get().Attached.Listen(func(struct{}) { v.Set(home.Firmware()) })
	v.Set(home.Firmware())
	return v
})

func Firmware() *firmwareview.View { return firmware() }

func screens() func(upright bool) firmwareview.Screen {
	screen := func(word string, device uint32, port int) firmwareview.Screen {
		return firmwareview.Screen{
			Dir: filepath.Join(layout.TesseraFirmwareDir, word),
			Relay: firmwarerelay.New(firmwarerelay.Options{
				Word:    word,
				Device:  device,
				Port:    port,
				Node:    func() string { return layout.Slug(config.Get().Device.Name) },
				Zone:    func() string { return timezone.Spec(config.Get().Time) },
				Home:    homeassistant.Get(),
				States:  states.Get(),
				Changed: func() { component.Reconnect.Emit(struct{}{}) },
			}),
		}
	}
	if !board.Current().Motion {
		dashboard := screen("dashboard", component.DeviceDashboard, layout.TesseraPort)
		return func(bool) firmwareview.Screen { return dashboard }
	}
	wide := screen("landscape", component.DeviceLandscape, layout.TesseraPort)
	tall := screen("portrait", component.DevicePortrait, layout.TesseraPort+1)
	return func(upright bool) firmwareview.Screen {
		if upright {
			return tall
		}
		return wide
	}
}
