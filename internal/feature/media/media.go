package media

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	sharedcard "github.com/ygelfand/libcountertop/pkg/display/mediacard"
	"github.com/ygelfand/libcountertop/pkg/media/mediaplayer"
	"github.com/ygelfand/libcountertop/pkg/media/nowplaying"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
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
		})
		theCard = sharedcard.New(
			sharedcard.Dependencies{Player: shared, Idle: config.IdleSection, Shell: shell.Get()},
		)
		shared.Begun.Listen(func(s nowplaying.Source) { theCard.Began(s) })
		drawer.Get().Add(theCard.Rail())
	})
	return shared
}

func (p *Player) Name() string { return "media player" }

func (p *Player) Entities() []esphome.Entity { return []esphome.Entity{p.Entity()} }

func (p *Player) Restore(config.Config) { p.Changed() }

func (p *Player) Run(ctx context.Context) error {
	p.Start(ctx)
	defer p.Close()
	defer card().Close()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}

		card().Follow()
		card().Tick()
	}
}

func (p *Player) SetIdle(v schema.Delay) {
	if err := config.Set().Idle().Media(v); err != nil {
		slog.Warn("saving the media idle timeout", "err", err)
	}
}

func (p *Player) Open() { card().Open() }

func (p *Player) refresh() {
	mp := p.Entity()
	mp.SetVolume(float32(volume.Get().Level(config.StreamMedia)) / 100)

	if mp.Muted() {
		speaker.Get().SetVolume(0)
	} else {
		volume.Get().Sounding(config.StreamMedia)
	}

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

func (mediaVolume) Mute(bool) {}
