package control

import (
	"fmt"
	"strconv"
	"strings"

	harness "github.com/ygelfand/libcountertop/pkg/runtime/control"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/cameracmd"
	"github.com/ygelfand/libcountertop/pkg/runtime/control/screencmd"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/a2dp"
	"github.com/ygelfand/LANovo/internal/feature/access"
	"github.com/ygelfand/LANovo/internal/feature/api"
	"github.com/ygelfand/LANovo/internal/feature/bluetooth"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/chromecast"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/microphone"
	"github.com/ygelfand/LANovo/internal/feature/network"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/rtspd"
	"github.com/ygelfand/LANovo/internal/feature/screen"
	"github.com/ygelfand/LANovo/internal/feature/sendspin"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/feature/weather"
)

type setting struct {
	name string

	field string

	says func(config.Config) string

	use func(string) error
}

func own() []setting {
	display := screen.Get()

	return append([]setting{
		{
			"media.duck",
			"Media.DuckDB",
			func(c config.Config) string { return strconv.FormatFloat(c.Media.DuckDB, 'f', -1, 64) },
			func(s string) error {
				db, err := strconv.ParseFloat(s, 64)
				if err != nil {
					return fmt.Errorf("ducking depth: %w", err)
				}
				return volume.Get().SetDuckDB(db)
			},
		},
		{
			"poster.on",
			"Poster.Enabled",
			func(c config.Config) string { return harness.OnOff(c.Poster.Enabled) },
			harness.Toggle(poster.Get().SetEnabled),
		},
		{
			"poster.every",
			"Poster.Every",
			func(c config.Config) string { return string(c.Poster.Every) },
			harness.Choose(schema.PosterEveries(), poster.Get().SetEvery),
		},
		{
			"poster.server",
			"Poster.Server",
			func(c config.Config) string { return harness.OrNone(c.Poster.Server) },
			harness.Words(poster.Get().SetServer),
		},
		{
			"poster.key",
			"Poster.Key",
			func(c config.Config) string { return harness.Secret(c.Poster.Key) },
			harness.Words(poster.Get().SetKey),
		},
		{
			"poster.albums",
			"Poster.Albums",
			func(c config.Config) string { return harness.OrNone(c.Poster.Albums) },
			harness.Words(poster.Get().SetAlbums),
		},
		{
			"poster.tags",
			"Poster.Tags",
			func(c config.Config) string { return harness.OrNone(c.Poster.Tags) },
			harness.Words(poster.Get().SetTags),
		},
		{
			"screen.backlight",
			"Screen.Backlight",
			func(c config.Config) string { return strconv.Itoa(c.Screen.Backlight) },
			harness.Number(0, 100, display.SetBacklight),
		},
		{
			"screen.mode",
			"Screen.Mode",
			func(c config.Config) string { return string(c.Screen.Mode) },
			harness.Choose(schema.ScreenModes(), display.SetMode),
		},
		{
			"cast.youtube.skip",
			"Cast.YouTube.Skip",
			func(c config.Config) string { return harness.OrNone(strings.Join(c.Cast.YouTube.Skip, ",")) },
			harness.Words(chromecast.Get().SetSkip),
		},
		{
			"cast.prime.persist",
			"Cast.Prime.Persist",
			func(c config.Config) string { return harness.OnOff(c.Cast.Prime.Persist) },
			harness.Toggle(chromecast.Get().SetPrimePersist),
		},
		{
			"cast.prime.skipintro",
			"Cast.Prime.SkipIntro",
			func(c config.Config) string { return harness.OnOff(c.Cast.Prime.SkipIntro) },
			harness.Toggle(chromecast.Get().SetPrimeSkipIntro),
		},
		{
			"cast.youtube.livedelay",
			"Cast.YouTube.LiveDelay",
			func(c config.Config) string { return strconv.Itoa(c.Cast.YouTube.LiveDelay) + " s" },
			harness.Number(
				schema.LiveDelayLeast,
				schema.LiveDelayMost,
				chromecast.Get().SetLiveDelay,
			),
		},
		{"cast.youtube.lounge", "Cast.YouTube.OnDemand", func(c config.Config) string {
			if c.Cast.YouTube.OnDemand {
				return "demand"
			}
			return "always"
		}, func(s string) error {
			switch s {
			case "always":
				chromecast.Get().SetLoungeOnDemand(false)
				return nil
			case "demand":
				chromecast.Get().SetLoungeOnDemand(true)
				return nil
			}
			return fmt.Errorf("want always or demand")
		}},
		{"microphone.gain", "Microphone.Gain", func(c config.Config) string {
			return strconv.Itoa(c.Microphone.Gain-config.DefaultMicGain) + " dB"
		}, harness.Number(0, microphone.GainMost, microphone.Get().SetGain)},
		{
			"microphone.leveling",
			"Microphone.Leveling",
			func(c config.Config) string { return harness.OnOff(c.Microphone.Leveling) },
			harness.Toggle(microphone.Get().SetLeveling),
		},
		{
			"microphone.denoise",
			"Microphone.Denoise",
			func(c config.Config) string { return harness.OnOff(c.Microphone.Denoise) },
			harness.Toggle(microphone.Get().SetDenoising),
		},
		{"microphone.visualizer_lift", "Microphone.VisualizerLift", func(c config.Config) string {
			return strconv.Itoa(c.Microphone.VisualizerLift) + " dB"
		}, harness.Number(0, visuals.LiftMax, microphone.Get().SetLift)},
		{"microphone.sensitivity", "Microphone.Sensitivity", func(c config.Config) string {
			return strconv.Itoa(c.Microphone.Sensitivity) + " dB"
		}, harness.Number(4, 20, microphone.Get().SetSensitivity)},
		{
			"features.sendspin",
			"Sendspin.Enabled",
			func(c config.Config) string { return harness.OnOff(c.Sendspin.Enabled) },
			harness.Toggle(sendspin.Get().SetEnabled),
		},
		{
			"call.incoming",
			"Call.Incoming",
			func(c config.Config) string { return harness.OnOff(c.Call.Incoming) },
			harness.Toggle(call.Get().SetIncoming),
		},
		{
			"call.auto_answer",
			"Call.AutoAnswer",
			func(c config.Config) string { return harness.OnOff(c.Call.AutoAnswer) },
			harness.Toggle(call.Get().SetAutoAnswer),
		},
		{
			"call.pause_wake",
			"Call.PauseWake",
			func(c config.Config) string { return harness.OnOff(c.Call.PauseWake) },
			harness.Toggle(call.Get().SetPauseWake),
		},
		{
			"call.auto_video",
			"Call.AutoVideo",
			func(c config.Config) string { return harness.OnOff(c.Call.AutoVideo) },
			harness.Toggle(call.Get().SetAutoVideo),
		},
		{
			"call.stream",
			"Call.Stream",
			func(c config.Config) string { return string(c.Call.Stream) },
			harness.Choose(schema.CallStreams(), call.Get().SetStream),
		},
		{
			"features.bluetooth",
			"Bluetooth.Proxy",
			func(c config.Config) string { return harness.OnOff(c.Bluetooth.Proxy) },
			harness.Toggle(bluetooth.Get().SetProxy),
		},
		{
			"features.speaker",
			"Bluetooth.Speaker",
			func(c config.Config) string { return harness.OnOff(c.Bluetooth.Speaker) },
			harness.Toggle(a2dp.Get().SetEnabled),
		},
		{
			"features.cast",
			"Cast.Receiver",
			func(c config.Config) string { return harness.OnOff(c.Cast.Receiver) },
			harness.Toggle(chromecast.Get().SetReceiver),
		},
		{
			"api.adopted",
			"API.Adopted",
			func(c config.Config) string { return harness.OnOff(c.API.Adopted) },
			harness.Toggle(api.Get().SetAdopted),
		},
		{
			"features.rtsp",
			"RTSP.Enabled",
			func(c config.Config) string { return harness.OnOff(c.RTSP.Enabled) },
			harness.Toggle(rtspd.Get().SetEnabled),
		},
		{
			"access.adb",
			"Access.ADB",
			func(c config.Config) string { return harness.OnOff(c.Access.ADB) },
			harness.Toggle(access.Get().SetADB),
		},
	}, append(presenceRows(), homeRows()...)...)
}

func presenceRows() []setting {
	rows := make([]setting, 0, len(sensors.Table().Rows()))
	for _, s := range sensors.Table().Rows() {
		field := "Presence.Wake"
		if s.Name == "range" {
			field = "Presence.Range"
		}
		rows = append(rows, setting{
			name:  "presence." + s.Name,
			field: field,
			says:  func(c config.Config) string { return s.Read(&c.Presence) },
			use:   func(v string) error { return sensors.SetPresence(s.Name, v) },
		})
	}
	return rows
}

func homeRows() []setting {
	var rows []setting
	for _, s := range homecontrol.Get().Selections() {
		rows = append(rows, setting{
			name:  "home.control." + s.Key,
			field: "Home.Control",
			says:  func(c config.Config) string { return harness.OnOff(c.Home.Control[s.Key]) },
			use:   harness.Toggle(s.SetControlled),
		}, setting{
			name:  "home.group." + s.Key,
			field: "Home.Group",
			says:  func(c config.Config) string { return strconv.Itoa(c.Home.Grouped(s.Key)) },
			use:   harness.Number(0, schema.HomeGroupMost, s.SetGroupFrom),
		})
	}
	rows = append(rows, setting{
		name:  "home.enabled",
		field: "Home.Enabled",
		says:  func(c config.Config) string { return harness.OnOff(c.Home.Enabled) },
		use:   harness.Toggle(homecontrol.Get().SetEnabled),
	})
	rows = append(rows, setting{
		name:  "home.combine",
		field: "Home.Combine",
		says:  func(c config.Config) string { return harness.OnOff(c.Home.Combine) },
		use:   harness.Toggle(homecontrol.Get().SetCombined),
	})
	return rows
}

func set(args []string) (string, error) { return harness.Settings(config.Get, settings(), args) }

func settings() []harness.Setting[config.Config] {
	rows := harness.Map(screencmd.Rows(screencmd.Dependencies{
		Clock:     dashboard.Get().Controls,
		Idle:      idle.Get().Idle,
		Visuals:   visuals.Get().Visuals,
		Weather:   weather.Get().Weather,
		Screen:    screen.Get().Screen,
		Settings:  config.ScreenSection,
		Shell:     shell.Get(),
		MediaIdle: media.Get().SetIdle,
		Marks:     privacy.Get().SetMarks,
		Verify:    network.Get().SetVerify,
	}), func(c config.Config) screencmd.State {
		return screencmd.State{
			Network: c.Network,
			Clock:   c.Clock,
			Idle:    c.Idle,
			Screen:  c.Screen,
			Visual:  c.Visual,
			Weather: c.Weather,
		}
	})
	rows = append(rows, harness.Map(
		cameracmd.Rows(livecam.Get().Camera),
		func(c config.Config) schema.Camera { return c.Camera },
	)...)
	for _, s := range own() {
		rows = append(rows, harness.Setting[config.Config]{
			Name:  s.name,
			Field: s.field,
			Read:  s.says,
			Write: s.use,
		})
	}
	return rows
}
