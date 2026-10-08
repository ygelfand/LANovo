package dashboard

import (
	"sync"

	sharedlib "github.com/ygelfand/libcountertop/pkg/display/dashboard"
	"github.com/ygelfand/libcountertop/pkg/display/geometry"
	"github.com/ygelfand/libcountertop/pkg/display/idleview"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	"github.com/ygelfand/libcountertop/pkg/display/theme"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(30))
}

type Dashboard struct{ *sharedlib.Controls }

var (
	once   sync.Once
	shared *Dashboard
)

func Get() *Dashboard {
	once.Do(func() {
		shared = &Dashboard{sharedlib.NewControls(sharedlib.Dependencies{
			Preferences: sharedlib.Preferences{
				Screen: config.ScreenSection,
				Clock:  config.ClockSection,
			},
			Display:  shell.Get(),
			DeviceID: component.DeviceScreen,
		})}

		screen.Get().Themed.Listen(func(theme.Theme) { shared.Redraw() })
		sensors.Get().Turned.Listen(func(geometry.Orientation) { shared.Redraw() })
		poster.Get().Changed.Listen(func(int) { shared.Redraw() })
		shell.Get().Changed.Listen(func(c sharedshell.Change) {
			if _, ok := c.To.(*idleview.View); ok {
				strip.Clock()
			}
		})
	})
	return shared
}

func (d *Dashboard) Name() string { return "dashboard" }

func (d *Dashboard) Restore(cfg config.Config) {
	d.Controls.Restore(sharedlib.StateOf(cfg.Screen, cfg.Clock))
}

func (d *Dashboard) Ready() bool {
	select {
	case <-clock.Get().Ready():
		return true
	default:
		return false
	}
}

func (d *Dashboard) Redraw() { shell.Get().Redraw() }
