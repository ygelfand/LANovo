package display

import (
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"
	"github.com/ygelfand/libcountertop/pkg/display/panel"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() {
	component.Register(sharedcomponent.Hardware, Get, sharedcomponent.Order(5),
		sharedcomponent.Supervise(service.Restart(time.Second, 30*time.Second)))
}

func Mounted() geometry.Orientation { return geometry.Orientation(board.Current().Mounted) }

func config() panel.Config {
	b := board.Current()
	return panel.Config{
		Framebuffer: layout.FBDevice,
		Socket:      layout.SurfaceSocket,
		Width:       b.PanelWidth,
		Height:      b.PanelHeight,
		Mounted:     Mounted(),
		Overlay:     panel.Overlay{Ion: layout.IonDevice, DispMgr: layout.DispMgrDevice},
	}
}

var get = sync.OnceValue(func() *panel.Driver { return panel.NewDriver(config) })

func Get() *panel.Driver { return get() }
