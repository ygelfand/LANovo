package gui

import (
	"sync"

	sharedvolume "github.com/ygelfand/libcountertop/pkg/audio/volume"
	"github.com/ygelfand/libcountertop/pkg/display/callview"
	"github.com/ygelfand/libcountertop/pkg/display/screens"
	"github.com/ygelfand/libcountertop/pkg/display/shell"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	tesseraview "github.com/ygelfand/libcountertop/pkg/tessera/widgets"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/assistant"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/firmware"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/settings"
	featureshell "github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/tessera"
	"github.com/ygelfand/LANovo/internal/feature/timer"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/feature/weather"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	hwtouch "github.com/ygelfand/LANovo/internal/hardware/touch"
	lanovoui "github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(36))
}

var Get = sync.OnceValue(func() *screens.App {
	look := widgets.NewUI(config.ScreenSection)
	calls := callview.New(callview.Options{
		UI:    look,
		Shell: featureshell.Get(),
		Calls: call.Get(),
		Peers: discovery.Get(),
		Voice: sharedvolume.For(volume.Get(), config.StreamVoice),
		Video: board.Current().CameraWidth > 0,
	})
	drawer.Get().Add(calls.Rail())
	return screens.New(screens.Options{
		Name:       "lanovo",
		UISize:     board.Current().UISize,
		DeviceName: func() string { return config.Get().Device.Name },
		Settings: func() screens.Settings {
			c := config.Get()
			return screens.Settings{
				Screen:  c.Screen,
				Clock:   c.Clock,
				Idle:    c.Idle,
				Weather: c.Weather,
			}
		},
		UI:       look,
		Weather:  config.WeatherSection,
		Brand:    theme.Brand,
		Logo:     lanovoui.Logo().Light,
		Night:    lanovoui.Logo().Dark,
		Offering: web.Get().Offering,

		Display:   display.Get(),
		Touch:     hwtouch.Get(),
		Turned:    &sensors.Get().Turned,
		Shell:     featureshell.Get(),
		Screen:    screen.Get().Screen,
		Messages:  message.Get(),
		Assistant: assistant.Get(),
		Stage:     assistant.Stage(),
		Timers:    timer.Get(),
		Poster:    poster.Get(),
		Drawer:    drawer.Get(),
		Tabs:      dashboard.Tabs(),
		Home:      homecontrol.Get(),
		HomeHA:    homeassistant.Get(),
		Saver:     idle.Get(),
		Tessera: tesseraview.New(tesseraview.Dependencies{
			UI:      look,
			Screens: tessera.Get(),
			Perform: homeassistant.Get().Perform,

			HomeAfterIdle: homecontrol.Get().ReturnHome,
			Logo:          lanovoui.Logo(),
		}),
		Forecast: weather.Get(),

		WeatherEntities: func() shell.View { return settings.WeatherEntities() },
		Player:          media.Get(),
		PlayerPage:      media.Page,
		MediaVolume:     sharedvolume.For(volume.Get(), config.StreamMedia),
		Volume:          volume.Get(),
		Streams:         config.Streams(),

		Upgrade: func() widgets.Upgrade {
			up := firmware.Get().Upgrade()
			return widgets.Upgrade{Active: up.Active(), At: up.At, Version: up.Version}
		},
		Upgrading: screens.On(&firmware.Get().Upgrading),
		Marks: func() screens.Marks {
			m := privacy.Get().Current()
			return screens.Marks{Mic: m.MicMuted, Camera: m.CameraBlocked}
		},
		Redraws: []screens.Watch{
			screens.On(&privacy.Get().Changed),
			screens.On(&web.Get().Offered),
		},

		Views: []func(shell.View) *screens.Screen{calls.Screen},
	})
})
