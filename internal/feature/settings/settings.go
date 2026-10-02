// Package settings is what the device can be set to, on the device itself.
//
// A screen is a title and a list of rows, and a row that leads somewhere pushes another screen.
// Drilling down rather than showing categories beside their contents: this panel is read from
// across a room, and two panes would halve everything on it.
package settings

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/lib/say"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

func init() {
	component.Register(component.Device, Get, component.Order(36))
}

// Settings is the feature. It holds nothing: every screen reads the config when it draws.
type Settings struct{}

var (
	once   sync.Once
	shared *Settings
)

func Get() *Settings {
	once.Do(func() {
		shared = &Settings{}

		// Before anything is labelled. The rail's names are resolved as it is built rather than as
		// it is drawn, so a language applied after this would leave them in the last one.
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

// Open puts the top of the settings up.
func Open() { shell.Get().Push(root()) }

func OpenCamera() {
	shell.Get().Push(root())
	shell.Get().Push(cameraPage())
}
