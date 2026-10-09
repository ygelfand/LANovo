package tessera

import (
	"path/filepath"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	shared "github.com/ygelfand/libcountertop/pkg/tessera"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() { component.Register(sharedcomponent.Network, Get, sharedcomponent.Order(95)) }

var get = sync.OnceValue(func() *shared.Set {
	home := homecontrol.Get()
	screen := func(f shared.Facing, device uint32, file string, on func() bool) *shared.Screen {
		return shared.New(shared.Options{
			Facing:   f,
			Device:   device,
			Board:    func() string { return board.Current().Tessera.Board },
			Size:     display.Get().Size,
			Grid:     home.Grid,
			Node:     func() string { return layout.Slug(config.Get().Device.Name) },
			Name:     func() string { return config.Get().Device.Name },
			Speaking: on,
			Keep:     filepath.Join(layout.TesseraDir, file),
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
	return set
})

func Get() *shared.Set { return get() }
