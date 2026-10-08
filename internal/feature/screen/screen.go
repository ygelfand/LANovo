package screen

import (
	"sync"

	libscreen "github.com/ygelfand/libcountertop/pkg/display/screen"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(10))
}

type Screen struct{ *libscreen.Screen }

var get = sync.OnceValue(func() *Screen {
	return &Screen{libscreen.New(libscreen.Options{
		Settings:  config.ScreenSection,
		DeviceID:  component.DeviceScreen,
		Backlight: func(level int) error { return display.Get().Brightness(level) },
		UISize:    func() string { return board.Current().UISize },
	})}
})

func Get() *Screen { return get() }

func Table() *libscreen.Table { return Get().Table() }

func (s *Screen) Restore(c config.Config) { s.Screen.Restore(c.Screen) }
