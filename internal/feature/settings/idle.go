package settings

import (
	sharedpages "github.com/ygelfand/libcountertop/pkg/display/settings"

	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/weather"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

func idlePages() *sharedpages.IdlePages {
	return sharedpages.NewIdlePages(
		sharedpages.IdleDependencies{
			Preferences: preferences(),
			Shell:       shell.Get(),
			Idle:        idle.Get(),
			Media:       media.Get(),
			Visuals:     visuals.Get(),
			Weather:     weather.Get(),
			Thumbnail:   visual.ThumbnailFit,
		},
	)
}

func WeatherPage() *shell.Page { return idlePages().Weather() }
