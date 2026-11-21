package airplay

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/volume"
)

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
