package chromecast

import (
	"context"

	"github.com/ygelfand/libcountertop/pkg/display/castscreen"
	"github.com/ygelfand/libcountertop/pkg/display/geometry"
	"github.com/ygelfand/libcountertop/pkg/display/video"
	"github.com/ygelfand/libcountertop/pkg/media/castoutput"
	"github.com/ygelfand/libcountertop/pkg/media/pcm"
	"github.com/ygelfand/libcountertop/pkg/media/videostream"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/videoplayer"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func newOutput() *castoutput.Output {
	a := speaker.Sound().Backgrounds()
	return castoutput.New(
		castoutput.Options{
			Queue:    speaker.Get(),
			Rate:     speaker.Rate,
			Channels: speaker.Channels,
			DuckDB:   func() float64 { return config.Get().Media.DuckDB },
			Media:    func() castoutput.Media { return media.Get() },
			Took:     func(p pcm.Producer) { a.Took(p) },
			Gave:     func(p pcm.Producer) { a.Gave(p) },
			Screen: castscreen.New(castscreen.Options{
				NewPage: videoplayer.NewPage,
				NewUnder: func() *videostream.Beneath {
					return &videostream.Beneath{
						Rotation: func() geometry.Orientation { return display.Get().Orientation() },
						Size:     func() (int, int) { return display.Get().Native() },
					}
				},
				Video: func(ctx context.Context, b *videostream.Beneath, s videostream.Stream) (videostream.Report, error) {
					player := video.Player{
						Helper:         display.Get().Helper(),
						SecureDecoders: board.Current().SecureDecoders,
					}
					return player.On(ctx, b, s)
				},
			}),
		},
	)
}
