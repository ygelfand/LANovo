package plex

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/audio/resample"
	"github.com/ygelfand/libcountertop/pkg/display/video"
	"github.com/ygelfand/libcountertop/pkg/media/playback"
	"github.com/ygelfand/libcountertop/pkg/media/plex"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/playout"
	"github.com/ygelfand/LANovo/internal/feature/product"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

var get = sync.OnceValue(func() *plex.Service {
	return plex.Build(product.Get(), config.PlexSection, plex.PlayerOptions{
		Output: playout.New(),
		Target: func() playback.Target {
			return video.Target(display.Get(), board.Current().MaxFPS)
		},
		Decoder: plex.Decoder{
			Width:  board.Current().DecodeWidth,
			Height: board.Current().DecodeHeight,
		},
		Resample: func(from int) func([]int16) []int16 {
			return resample.NewRational(from, speaker.Rate, speaker.Channels).Run
		},
	}, config.Get().Device.Name)
})

func Get() *plex.Service { return get() }
