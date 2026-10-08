package media

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/libcountertop/pkg/hook"
	"github.com/ygelfand/libcountertop/pkg/media/ownership"
	"github.com/ygelfand/libcountertop/pkg/media/pcm"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Device, Get, component.Order(35),
		component.Supervise(service.Restart(5*time.Second, time.Minute)))
}

type Player struct {
	Begun hook.Hook[Source]
	mp    *esphome.MediaPlayer

	stream *pcm.Stream

	speaking atomic.Bool

	owner     *ownership.Owner[Source]
	ownerInit sync.Once
}

var (
	once   sync.Once
	shared *Player
)

func Get() *Player {
	once.Do(func() {
		shared = &Player{}
		shared.build()
		onRail()
	})
	return shared
}

func (p *Player) Name() string { return "media player" }

func (p *Player) Entities() []esphome.Entity { return []esphome.Entity{p.mp} }

func (p *Player) Restore(config.Config) { p.refresh() }

func (p *Player) build() {
	p.mp = &esphome.MediaPlayer{
		Base: esphome.Base{ObjectID: "speaker", Name: "Speaker", Icon: "mdi:speaker"},
		Features: esphome.MediaPlayerFeatureVolumeSet |
			esphome.MediaPlayerFeatureVolumeStep |
			esphome.MediaPlayerFeatureVolumeMute |
			esphome.MediaPlayerFeaturePlayMedia |
			esphome.MediaPlayerFeatureBrowseMedia |
			esphome.MediaPlayerFeaturePlay |
			esphome.MediaPlayerFeaturePause |
			esphome.MediaPlayerFeatureStop |
			esphome.MediaPlayerFeatureAnnounce,
		SupportedFormats: Formats,
		SupportsPause:    true,
	}

	p.mp.OnCommand = p.command
	arb := speaker.Sound().Backgrounds()
	p.stream = pcm.New(pcm.Options{
		Sink: speaker.Get(), Rate: speaker.Rate, Channels: speaker.Channels, Changed: p.refresh,
		DuckDB:      func() float64 { return config.Get().Media.DuckDB },
		Arbitration: arb,
	})
}

func (p *Player) Run(ctx context.Context) error {
	p.stream.Start(ctx)
	defer p.stream.Close()
	defer card().Close()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}

		p.follow()
		p.tick()
	}
}

func (p *Player) tick() { card().Tick() }

func (p *Player) Began(s Source) {
	replaced, fresh, displaced := p.sourceOwner().Begin(s)
	if fresh {
		card().Began(s)
		p.Begun.Emit(s)
	}
	p.refresh()
	if displaced {
		replaced.Stop()
	}
}

func (p *Player) Ended(s Source) { p.sourceOwner().End(s) }

func (p *Player) follow() { card().Follow() }

func (p *Player) SetIdle(v config.Delay) {
	if err := config.Set().Idle().Media(v); err != nil {
		slog.Warn("saving the media idle timeout", "err", err)
	}
}

func (p *Player) Open() { card().Open() }

func (p *Player) Pause() {
	if source := p.source(); source != nil {
		source.Pause()
		return
	}
	p.stream.Pause()
}
func (p *Player) command(c esphome.MediaCommand) {
	if c.HasVolume {
		volume.Get().Set(config.StreamMedia, int(math.Round(float64(c.Volume)*100)))
		p.refresh()
	}

	if c.HasMediaURL && c.MediaURL != "" {
		p.play(c.MediaURL, c.Announcement)
	}

	if c.HasCommand && c.Command == esphome.MediaPlayerStop {
		p.Ended(homeAssistant{})
	}

	if !c.HasCommand {
		return
	}

	if s := p.source(); s != nil {
		switch c.Command {
		case esphome.MediaPlayerPlay:
			s.Play()
			return
		case esphome.MediaPlayerPause:
			s.Pause()
			return
		case esphome.MediaPlayerStop:
			s.Stop()
			return
		}
	}

	switch c.Command {
	case esphome.MediaPlayerVolumeUp:
		p.adjust(volume.Step)
	case esphome.MediaPlayerVolumeDown:
		p.adjust(-volume.Step)
	case esphome.MediaPlayerMute:
		p.Mute(true)
	case esphome.MediaPlayerUnmute:
		p.Mute(false)
	case esphome.MediaPlayerStop:
		p.Stop()
	}
}

// Home Assistant serves the url as WAV, at the card's rate for music and the pipeline's for an announcement.
func (p *Player) play(url string, announcement bool) {
	if announcement {
		p.announce(url)
		return
	}
	p.Began(homeAssistant{})
	p.stream.Play(url)
}
func (p *Player) announce(url string) {
	p.Sounding(true)
	claim := speaker.Sound().
		Claim("announce", func(ctx context.Context, spk *speaker.Speaker) error {
			samples, err := Fetch(ctx, url)
			if err != nil {
				return err
			}
			spk.PlayVoice(samples)
			spk.PlayVoice(make([]int16, speaker.VoiceRate*Tail/1000))
			return nil
		})
	safe.Go("announce", func() {
		<-claim.Done()
		p.Sounding(false)
		if err := claim.Err(); err != nil {
			slog.Error("playing announcement failed", "err", err)
		}
	})
}

func (p *Player) Stop() { p.stream.Stop(); p.refresh() }

func (p *Player) Sounding(on bool) {
	p.speaking.Store(on)
	p.refresh()
}

func (p *Player) Playing() (playing, paused bool) {
	playing, paused = p.stream.Playing()
	if s := p.source(); s != nil {
		n := s.Now()
		playing = playing || n.Playing && !n.Paused
		paused = paused || n.Paused
	}
	return
}
func (p *Player) adjust(by int) {
	v := volume.Get()
	v.Set(config.StreamMedia, v.Level(config.StreamMedia)+by)
	p.refresh()
}

func (p *Player) Mute(on bool) {
	p.mp.SetMuted(on)
	p.refresh()
}

func (p *Player) refresh() {
	p.mp.SetVolume(float32(volume.Get().Level(config.StreamMedia)) / 100)

	if p.mp.Muted() {
		speaker.Get().SetVolume(0)
	} else {
		volume.Get().Sounding(config.StreamMedia)
	}

	now := p.Now()
	switch {
	case now.Playing || p.speaking.Load():
		p.mp.SetState(esphome.MediaPlayerPlaying)
	case now.Paused:
		p.mp.SetState(esphome.MediaPlayerPaused)
	default:
		p.mp.SetState(esphome.MediaPlayerIdle)
	}

	if Showing() {
		shell.Get().Redraw()
	}
}

func (p *Player) sourceOwner() *ownership.Owner[Source] {
	p.ownerInit.Do(func() {
		p.owner = ownership.New(func(s Source) uint8 { return uint8(s.Kind()) })
	})
	return p.owner
}
