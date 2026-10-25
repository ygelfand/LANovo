package media

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/media/mediaplayer"
	"github.com/ygelfand/libcountertop/pkg/media/noiselayer"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(35),
		sharedcomponent.Supervise(service.Restart(5*time.Second, time.Minute)))
}

type Player struct {
	*mediaplayer.Player[*speaker.Speaker]

	layers *noiselayer.Layers
}

var (
	once   sync.Once
	shared *Player
)

func Get() *Player {
	once.Do(func() {
		shared = &Player{}
		shared.Player = mediaplayer.New(mediaplayer.Options[*speaker.Speaker]{
			Speaker:  speaker.Get(),
			Sound:    speaker.Sound(),
			Rate:     speaker.Rate,
			Channels: speaker.Channels,
			DuckDB:   func() float64 { return config.Get().Media.DuckDB },
			Volume:   mediaVolume{},
			Changed:  shared.refresh,
			Device:   component.DevicePlayback,
		})
		shared.layers = noiselayer.New(shared.Stream(), component.DevicePlayback)
		volume.Get().Watch(config.StreamMedia, func(int) { shared.Changed() })
	})
	return shared
}

func (p *Player) Name() string { return "media player" }

func (p *Player) Entities() []esphome.Entity {
	out := append([]esphome.Entity{p.Entity()}, p.Track()...)
	return append(out, p.layers.Entities()...)
}

func (p *Player) Restore(config.Config) { p.Changed() }

func (p *Player) Run(ctx context.Context) error {
	p.Start(ctx)
	defer p.Close()
	<-ctx.Done()
	return nil
}

func (p *Player) SetIdle(v schema.Delay) {
	if err := config.Set().Idle().Media(v); err != nil {
		slog.Warn("saving the media idle timeout", "err", err)
	}
}

func (p *Player) refresh() {
	mp := p.Entity()
	mp.SetVolume(float32(volume.Get().Level(config.StreamMedia)) / 100)
	mp.SetMuted(volume.Get().Muted(config.StreamMedia))
	p.layers.Sync()

	if Showing() {
		shell.Get().Redraw()
	}
}

type mediaVolume struct{}

func (mediaVolume) SetVolume(fraction float32) {
	volume.Get().Set(config.StreamMedia, int(math.Round(float64(fraction)*100)))
}

func (mediaVolume) StepVolume(delta int) {
	v := volume.Get()
	v.Set(config.StreamMedia, v.Level(config.StreamMedia)+delta*volume.Step)
}

func (mediaVolume) Mute(on bool) { volume.Get().Mute(config.StreamMedia, on) }
