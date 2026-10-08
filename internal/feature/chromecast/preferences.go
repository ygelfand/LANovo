package chromecast

import (
	"github.com/ygelfand/libcountertop/pkg/media/cast/preferences"

	"github.com/ygelfand/LANovo/internal/config"
)

func init() {
	preferences.Configure(preferences.Provider{
		RequestIDPrefix: "lanovo",
		Read: func() preferences.Settings {
			c := config.Get().Cast
			return preferences.Settings{
				YouTube: preferences.YouTube{
					Device:    c.YouTube.Device,
					Music:     c.YouTube.Music,
					Video:     c.YouTube.Video,
					Skip:      c.YouTube.Skip,
					OnDemand:  c.YouTube.OnDemand,
					LiveDelay: c.YouTube.LiveDelay,
				},
				Prime: preferences.Prime{Persist: c.Prime.Persist, SkipIntro: c.Prime.SkipIntro},
			}
		},
		SaveScreens: func(device, music, video string) error { return config.Set().Cast().Screens(device, music, video) },
	})
}
