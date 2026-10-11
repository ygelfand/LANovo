package drawer

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/drawer"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/screen"
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
		Hide:  func() schema.Delay { return config.Get().Screen.DockHide },
	})
	hwtouch.Get().Gestures.Listen(r.Gesture)
	shell.Get().Changed.Listen(func(sharedshell.Change) { r.Settle() })
	screen.Get().Restyled.Listen(func(schema.Screen) { r.Settle() })
	r.Settle()
	return r
})

func Get() *drawer.Rail { return get() }
