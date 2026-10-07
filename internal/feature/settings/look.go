package settings

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/ui/visual"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
)

func lookPages() *sharedsettings.LookPages {
	return sharedsettings.NewLookPages(
		sharedsettings.LookOptions{
			Read:      func(slot int) config.Look { return config.Get().Wake.Slot(slot).Look },
			Place:     func(slot int, v config.LookPlace) error { return config.Set().Wake(slot).Place(v) },
			Visual:    func(slot int, st config.Stage, kind string) error { return config.Set().Wake(slot).Visual(st, kind) },
			Push:      shell.Get().Push,
			Pop:       shell.Get().Pop,
			Thumbnail: visual.ThumbnailFit,
		},
	)
}
func lookPage(slot int) *shell.Page                   { return lookPages().Page(slot) }
func placePage(slot int) *shell.Page                  { return lookPages().Place(slot) }
func stagePage(slot int, st config.Stage) *shell.Page { return lookPages().Stage(slot, st) }

var lookName = sharedsettings.LookName
