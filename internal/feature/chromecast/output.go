package chromecast

import (
	"context"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/videoplayer"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/hardware/video"
	"github.com/ygelfand/libcountertop/pkg/media/castoutput"
	"github.com/ygelfand/libcountertop/pkg/media/pcm"
)

const Queued = castoutput.Queued

type output = castoutput.Output

func newOutput() *output {
	a := speaker.Sound().Backgrounds()
	return castoutput.New(castoutput.Options{Queue: speaker.Get(), Rate: speaker.Rate, Channels: speaker.Channels, DuckDB: func() float64 { return config.Get().Media.DuckDB }, Media: func() castoutput.Media { return media.Get() }, Took: func(p pcm.Producer) { a.Took(p) }, Gave: func(p pcm.Producer) { a.Gave(p) }, NewPage: videoplayer.NewPage, NewUnder: func() *video.Beneath {
		return &video.Beneath{Rotation: func() display.Orientation { return display.Get().Orientation() }, Size: func() (int, int) { return display.Get().Native() }}
	}, Video: func(ctx context.Context, b *video.Beneath, s video.Stream) (video.Report, error) {
		return video.On(ctx, b, s)
	}})
}
