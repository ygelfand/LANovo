package settings

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/say"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

func init() {
	component.Register(component.Device, Get, component.Order(36))
}

type Settings struct{}

var (
	once   sync.Once
	shared *Settings
)

func Get() *Settings {
	once.Do(func() {
		shared = &Settings{}

		say.Use(config.Get().Screen.Language)

		drawer.Get().Add(drawer.Entry{
			Name:  func() string { return say.T("settings.title") },
			Order: drawer.OrderSettings,
			Glyph: func() string { return gogui.IconGear },
			Open:  Open,
		})
	})
	return shared
}

func (s *Settings) Name() string { return "settings" }

func Open() { shell.Get().Push(root()) }

func OpenCamera() {
	shell.Get().Push(root())
	shell.Get().Push(cameraPage())
}
