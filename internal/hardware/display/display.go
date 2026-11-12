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

func Mounted() geometry.Orientation {
	b := board.Current()
	if b.Name == "sd-x501f" {
		// The display helper and LANovo both use the observed 1280x800
		// landscape coordinate space after Android Things' projection.
		return geometry.Rotate0
	}
	return geometry.Orientation(b.Mounted)
}

var helperSocket = layout.SurfaceSocket

// SetHelperSocket selects the socket before the display service starts.
func SetHelperSocket(path string) { helperSocket = path }

func config() panel.Config {
	b := board.Current()
	surfaceZ := 0
	width, height, mounted := b.PanelWidth, b.PanelHeight, Mounted()
	if b.Name == "sd-x501f" {
		// Place the SD-X501F SurfaceFlinger layer above Android's application layers.
		surfaceZ = 1_000_000
		// SurfaceFlinger exposes the rotated landscape coordinate space.
		width, height, mounted = b.PanelHeight, b.PanelWidth, geometry.Rotate0
	}
	return panel.Config{
		Framebuffer: layout.FBDevice,
		Socket:      helperSocket,
		Width:       width,
		Height:      height,
		Mounted:     mounted,
		SurfaceZ:    surfaceZ,
		Overlay:     panel.Overlay{Ion: layout.IonDevice, DispMgr: layout.DispMgrDevice},
	}
}

var get = sync.OnceValue(func() *panel.Driver { return panel.NewDriver(config) })

func Get() *panel.Driver { return get() }
