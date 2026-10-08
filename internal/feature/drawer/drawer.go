package drawer

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/drawer"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	hwtouch "github.com/ygelfand/LANovo/internal/hardware/touch"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(35))
}

var get = sync.OnceValue(func() *drawer.Rail {
	r := drawer.New(drawer.Options{
		Shell: shell.Get(),
		Edge:  func() schema.Edge { return config.Get().Screen.Drawer },
	})
	hwtouch.Get().Gestures.Listen(r.Gesture)
	return r
})

func Get() *drawer.Rail { return get() }
