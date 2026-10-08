package voice

import (
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/assistant/satellite"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/timer"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/wake"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(30))
}

const Features = satellite.Features

type Voice struct{ *satellite.Satellite }

var (
	once   sync.Once
	shared *Voice
)

func Get() *Voice {
	once.Do(func() { shared = build() })
	return shared
}

func build() *Voice {
	ours := wake.Lib().Ours()
	slots := wanted(ours, wakeword.Slots)

	v := &Voice{satellite.New(
		&esphome.VoiceSatellite{OnExternalWakeWords: wake.Lib().Answer},
		dependencies(),
		satellite.Options{
			Slots:   wakeword.Slots,
			Initial: slots,
			Timers:  timer.Get(),
			Sound:   speaker.Sound(),
			Player:  media.Get(),
			Save:    func(slot int, id string) error { return config.Set().Wake(slot).ID(id) },
		},
	)}
	slog.Info("wake words", "ours", len(ours), "slots", slots)

	wakeword.Requested.Listen(v.Start)

	return v
}
