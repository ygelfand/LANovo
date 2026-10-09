package call

import (
	"context"

	"github.com/ygelfand/libcountertop/pkg/audio/tone"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

type device struct{}

func (device) Frames() (<-chan []int16, func()) { return mic.Get().Listen("call") }

func (device) Play(pcm []int16) {
	speaker.Get().PlayStream(config.StreamVoice, pcm)
}

func (device) Queued() int { return speaker.Get().Queued() }

func (device) Claim(name string, run func(context.Context) error) {
	speaker.Sound().
		Claim(name, func(ctx context.Context, _ *speaker.Speaker) error { return run(ctx) })
}
func (device) Duck(on bool) { speaker.Sound().Backgrounds().Duck(on) }
func (device) Chime(s config.Stream, level float64, notes ...tone.Note) {
	speaker.Sound().Interject(func(p *speaker.Speaker) { p.Chime(s, level, notes...) })
}
