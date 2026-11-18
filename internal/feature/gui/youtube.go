package gui

import (
	"github.com/ygelfand/libcountertop/pkg/audio/resample"
	"github.com/ygelfand/libcountertop/pkg/display/musicapp"
	"github.com/ygelfand/libcountertop/pkg/display/video"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
	"github.com/ygelfand/libcountertop/pkg/display/youtubeapp"
	"github.com/ygelfand/libcountertop/pkg/fetch"
	"github.com/ygelfand/libcountertop/pkg/media/playback"
	"github.com/ygelfand/libcountertop/pkg/media/youtube"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/playout"
	featureshell "github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func youTube(look *widgets.UI) *youtubeapp.App {
	client := fetch.Client(musicapp.FetchTime)
	r := youtube.NewResolver(client, func() playback.Target {
		return video.Target(display.Get(), board.Current().MaxFPS)
	})
	r.Resample = func(from int) func([]int16) []int16 {
		return resample.NewRational(from, speaker.Rate, speaker.Channels).Run
	}
	return youtubeapp.New(youtubeapp.Options{
		UI:       look,
		Shell:    featureshell.Get(),
		Accounts: config.YouTubeAccountsSection,
		Resolver: r,
		Output:   playout.New(),
		Skip:     func() []string { return config.Get().Cast.YouTube.Skip },
		Client:   client,
		Redraw:   func() { featureshell.Get().Redraw() },
	})
}
