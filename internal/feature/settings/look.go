package settings

import (
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"

	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

func lookPages() *sharedsettings.LookPages {
	return sharedsettings.NewLookPages(
		sharedsettings.LookDependencies{
			Preferences: preferences(),
			Shell:       shell.Get(),
			Thumbnail:   visual.ThumbnailFit,
		},
	)
}
