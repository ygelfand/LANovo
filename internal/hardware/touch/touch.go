package touch

import (
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"
	"github.com/ygelfand/libcountertop/pkg/input/touch"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

func init() {
	component.Register(sharedcomponent.Hardware, Get, sharedcomponent.Order(30),
		sharedcomponent.Supervise(service.Restart(time.Second, time.Minute)))
}

type panel struct{}

func (panel) Native() (w, h int) {
	return board.Current().PanelWidth, board.Current().PanelHeight
}

func (panel) Orientation() geometry.Orientation {
	return touchOrientation(board.Current(), display.Get().Orientation())
}

func touchOrientation(b board.Board, orientation geometry.Orientation) geometry.Orientation {
	if b.Name == board.X501F.Name {
		// The Goodix controller reports the panel in its native 800x1280
		// coordinate space while SurfaceFlinger presents a landscape 1280x800
		// screen. Map touch points independently of the compositor orientation.
		return geometry.Orientation((90 + int(orientation)) % 360)
	}
	return orientation
}

var get = sync.OnceValue(func() *touch.Screen {
	return touch.New(device(), panel{})
})

func device() string {
	names := board.Current().Touch
	for _, name := range names {
		if _, err := touch.Find(name); err == nil {
			return name
		}
	}
	return names[0]
}

func Get() *touch.Screen { return get() }
