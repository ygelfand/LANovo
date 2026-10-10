package chromecast

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/audio/resample"
	"github.com/ygelfand/libcountertop/pkg/display/surface"
	"github.com/ygelfand/libcountertop/pkg/display/video"
	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"github.com/ygelfand/libcountertop/pkg/media/cast/protocols/primevideo"
	"github.com/ygelfand/libcountertop/pkg/media/cast/protocols/unsupported"
	"github.com/ygelfand/libcountertop/pkg/media/cast/protocols/youtube"
	"github.com/ygelfand/libcountertop/pkg/media/cast/protocols/youtube/unplugged"
	"github.com/ygelfand/libcountertop/pkg/media/castreceiver"
	"github.com/ygelfand/libcountertop/pkg/media/playback"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/dhcp"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/ui"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(60))
}

var Preferences = castreceiver.Preferences("lanovo", config.CastSection)

var get = sync.OnceValue(build)

func Get() *castreceiver.Receiver { return get() }

func helper() *surface.Client { return display.Get().Helper() }

func build() *castreceiver.Receiver {
	r := castreceiver.NewReceiver(castreceiver.ReceiverOptions{
		Settings: config.CastSection,
		Network:  func(ctx context.Context) bool { return dhcp.Get().Wait(ctx) },
		Server: castreceiver.Config{
			Name:            deviceName,
			Model:           layout.Model,
			Version:         layout.Version,
			Hardware:        hardware,
			Host:            layout.Slug,
			Logo:            ui.Logo().PNG,
			Address:         func() string { return config.Get().Network.Address },
			Timezone:        func() string { return config.Get().Time.Chosen },
			CredentialsPath: layout.CastCredentialsPath,
			AuthorityPath:   layout.CastAuthorityPath,
			AppDir:          layout.CastAppDir,
			Synced: func(do func()) func() {
				return clock.Get().Synced.Listen(func(time.Time) { do() })
			},
			Volume: mediaVolume{},
			Video: func() playback.Target {
				return video.Target(display.Get(), board.Current().MaxFPS)
			},
			Output: func(ended func()) playback.Output {
				o := newOutput()
				o.SetEnded(ended)
				return o
			},
			Resample: func(from int) func([]int16) []int16 {
				return resample.NewRational(from, speaker.Rate, speaker.Channels).Run
			},
			Apps: []cast.Maker{
				primevideo.Maker(helper),
				unsupported.Maker,
				youtube.Maker(unplugged.TV(helper)),
			},
			Preferences: Preferences,
		},
	})
	component.Settings.Add(r.Settings())
	return r
}

type mediaVolume struct{}

func (mediaVolume) Level() int { return volume.Get().Level(config.StreamMedia) }

func (mediaVolume) SetLevel(level int) { volume.Get().Set(config.StreamMedia, level) }

func (mediaVolume) Mute(on bool) { volume.Get().Mute(config.StreamMedia, on) }

func (mediaVolume) Muted() bool { return volume.Get().Muted(config.StreamMedia) }

func (mediaVolume) Listen(changed func(int)) func() {
	return volume.Get().Changed.Listen(func(c volume.Change) {
		if c.Stream == config.StreamMedia {
			changed(c.Level)
		}
	})
}

func deviceName() string {
	if name := config.Get().Device.Name; name != "" {
		return name
	}
	return layout.DefaultName
}

func hardware() string {
	raw, err := os.ReadFile(layout.MACPath)
	if err != nil {
		slog.Warn("the cast identity has no hardware address to come from", "err", err)
		return ""
	}
	return layout.MAC(string(raw))
}
