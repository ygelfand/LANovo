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

func (panel) Orientation() geometry.Orientation { return display.Get().Orientation() }

var get = sync.OnceValue(func() *touch.Screen {
	return touch.New(board.Current().Touch, panel{})
})

func Get() *touch.Screen { return get() }
