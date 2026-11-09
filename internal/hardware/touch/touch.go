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
	name := device(board.Current().Touch, touch.Find)
	if name == board.AmberElanTouch {
		return touch.NewWithAxisNormalization(name, panel{})
	}
	return touch.New(name, panel{})
})

func device(names []string, find func(string) (string, error)) string {
	for _, name := range names {
		if _, err := find(name); err == nil {
			return name
		}
	}
	return names[0]
}

func Get() *touch.Screen { return get() }
