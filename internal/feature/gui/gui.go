package gui

import (
	"sync"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"
	sharedalarm "github.com/ygelfand/libcountertop/pkg/assistant/alarm"
	sharedvolume "github.com/ygelfand/libcountertop/pkg/audio/volume"
	"github.com/ygelfand/libcountertop/pkg/display/agenda"
	"github.com/ygelfand/libcountertop/pkg/display/alarms"
	"github.com/ygelfand/libcountertop/pkg/display/callview"
	"github.com/ygelfand/libcountertop/pkg/display/dawnview"
	"github.com/ygelfand/libcountertop/pkg/display/mirror"
	"github.com/ygelfand/libcountertop/pkg/display/radarview"
	"github.com/ygelfand/libcountertop/pkg/display/screens"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
	"github.com/ygelfand/libcountertop/pkg/display/shell"
	"github.com/ygelfand/libcountertop/pkg/display/shopping"
	"github.com/ygelfand/libcountertop/pkg/display/syncview"
	"github.com/ygelfand/libcountertop/pkg/display/tuner"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	tesseraview "github.com/ygelfand/libcountertop/pkg/tessera/widgets"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/alarm"
	"github.com/ygelfand/LANovo/internal/feature/alerts"
	"github.com/ygelfand/LANovo/internal/feature/apps"
	"github.com/ygelfand/LANovo/internal/feature/assistant"
	"github.com/ygelfand/LANovo/internal/feature/calendar"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/cameras"
	"github.com/ygelfand/LANovo/internal/feature/commands"
	"github.com/ygelfand/LANovo/internal/feature/configsync"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/firmware"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/lock"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/feature/nws"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/radar"
	"github.com/ygelfand/LANovo/internal/feature/radio"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/settings"
	featureshell "github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/tessera"
	"github.com/ygelfand/LANovo/internal/feature/timer"
	"github.com/ygelfand/LANovo/internal/feature/todo"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
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
	apps.Get().Add(calls.App())
	apps.Get().Add(cameras.Get().App())
	var mirrors []func(shell.View) *screens.Screen
	if board.Current().CameraWidth > 0 {
		m := mirror.New(mirror.Options{Shell: featureshell.Get(), Camera: settings.Preview()})
		apps.Get().Add(m.App())
		commands.Get().Listen(m.Command())
		mirrors = append(mirrors, m.Screen)
	}
	syncs := syncview.New(syncview.Options{
		UI:    look,
		Shell: featureshell.Get(),
		Sync:  configsync.Get(),
		Peers: discovery.Get(),
		Logo:  sharedsettings.Themed(lanovoui.Logo().Light, lanovoui.Logo().Dark),
	})
	var app *screens.App
	shop := shopping.New(shopping.Options{
		UI:    look,
		Shell: featureshell.Get(),
		Todo:  todo.Get().Todo,
		Gate:  func(w *gogui.Window) gogui.View { return app.Gate(w) },
	})
	apps.Get().Add(shop.App())
	diary := agenda.New(agenda.Options{
		UI:       look,
		Shell:    featureshell.Get(),
		Calendar: calendar.Get().Calendar,
		Gate:     func(w *gogui.Window) gogui.View { return app.Gate(w) },
		TwentyFour: func() bool {
			return config.Get().Screen.Hours == schema.TwentyFourHour
		},
	})
	apps.Get().Add(diary.App())
	dial := tuner.New(tuner.Options{
		UI:     look,
		Shell:  featureshell.Get(),
		Radio:  radio.Get(),
		Player: media.Get(),
		Gate:   func(w *gogui.Window) gogui.View { return app.Gate(w) },
		Redraw: func() { featureshell.Get().Redraw() },
	})
	apps.Get().Add(dial.App())
	sky := radarview.New(radarview.Options{
		UI:       look,
		Shell:    featureshell.Get(),
		Radar:    radar.Get(),
		Settings: config.WeatherSection,
		Clock24: func() bool {
			return config.Get().Screen.Hours == schema.TwentyFourHour
		},
		Again: func(w *gogui.Window, after time.Duration) { app.Again(w, after) },
		Gate:  func(w *gogui.Window) gogui.View { return app.Gate(w) },
	})
	apps.Get().Add(sky.App())
	commands.Get().Listen(sky.Command())
	wake := alarms.New(alarms.Options{
		Shell:  featureshell.Get(),
		Alarms: alarm.Get(),
		Tones:  alerts.Get().Tones,
	})
	apps.Get().Add(wake.App())
	light := dawnview.NewLight(
		featureshell.Get(),
		alarm.Get(),
		visuals.Show(dawnview.NewSource(dawnview.Options{
			Alarms:  alarm.Get(),
			Look:    func() schema.WakeLook { return alerts.Get().Current().Wake },
			Clock24: func() bool { return config.Get().Screen.Hours == schema.TwentyFourHour },
		})),
		idle.Get(),
	)
	alarm.Get().Dawning.Listen(func(sharedalarm.Dawn) { light.Check() })
	alarm.Get().Changed.Listen(func(struct{}) {
		if alarm.Get().Ringing() {
			idle.Get().Wake()
		}
		light.Check()
		featureshell.Get().Redraw()
	})
	sensors.Get().Rising(func() (float64, bool) {
		if d := alarm.Get().Dawn(); d.Rising() {
			return d.Share(time.Now()), true
		}
		return 1, dawnview.Showing(alarm.Get())
	})
	app = screens.New(screens.Options{
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
		Forecast:  weather.Get(),
		Warnings:  nws.Get(),
		Alarms:    alarm.Get(),
		OpenRadar: sky.Open,

		WeatherEntities: func() shell.View { return settings.WeatherEntities() },
		Todo:            todo.Get().Todo,
		Calendar:        calendar.Get().Calendar,
		CamerasPage:     func() shell.View { return cameras.Get().Page() },
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
			screens.On(&firmware.Get().Offered),
			screens.On(&firmware.Get().Listed),
			screens.On(&privacy.Get().Changed),
			screens.On(&web.Get().Offered),
		},

		Views: append([]func(shell.View) *screens.Screen{
			calls.Screen,
			syncs.Screen,
			lock.Get().Gate().Views(look),
			shop.Screen,
			diary.Screen,
			dial.Screen,
			sky.Screen,
			cameras.Get().Screen,
			apps.Get().Screens(look),
		}, mirrors...),
	})
	return app
})
