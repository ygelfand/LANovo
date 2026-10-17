package airplay

import (
	"image"
	"log/slog"
	"os"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"
	sharedairplay "github.com/ygelfand/libcountertop/pkg/media/airplay"
	"github.com/ygelfand/libcountertop/pkg/media/playback"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/castvideo"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/parts"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(61))
}

type Receiver struct{ *sharedairplay.Receiver }

func (r *Receiver) Restore(cfg config.Config) { r.Receiver.Restore(cfg.AirPlay) }

func (r *Receiver) Close() error {
	r.Stop()
	return nil
}

var get = sync.OnceValue(func() *Receiver {
	r := &Receiver{sharedairplay.NewReceiver(sharedairplay.Options{
		Settings: config.AirPlaySection,
		Device:   component.DevicePlayback,
		Helper:   layout.AirPlayHelper,
		Binary:   parts.AirPlay(),
		Keyfile:  layout.AirPlayKey,
		Name:     deviceName,
		MAC:      mac,
		Size: func() (int, int) {
			w, h := display.Get().Native()
			return display.Get().Orientation().Size(w, h)
		},
		Largest: image.Pt(board.Current().DecodeWidth, board.Current().DecodeHeight),
		Output: func(ended func()) playback.Output {
			o := castvideo.New()
			o.SetEnded(ended)
			return o
		},
		Rate:     speaker.Rate,
		Channels: speaker.Channels,
	})}
	component.Settings.Add(r.Settings())
	sensors.Get().Turned.Listen(func(geometry.Orientation) { r.Resize() })
	return r
})

func Get() *Receiver { return get() }

func deviceName() string {
	if name := config.Get().Device.Name; name != "" {
		return name
	}
	return layout.DefaultName
}

func mac() string {
	raw, err := os.ReadFile(layout.MACPath)
	if err != nil {
		slog.Warn("airplay has no hardware address", "err", err)
		return ""
	}
	return layout.MAC(string(raw))
}
