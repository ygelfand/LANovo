package settings

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/ui/visual"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
)

func lookPages() *sharedsettings.LookPages {
	return sharedsettings.NewLookPages(sharedsettings.LookDependencies{Preferences: preferences(), Shell: shell.Get(), Thumbnail: visual.ThumbnailFit})
}
func lookPage(slot int) *shell.Page                   { return lookPages().Page(slot) }
func placePage(slot int) *shell.Page                  { return lookPages().Place(slot) }
func stagePage(slot int, st config.Stage) *shell.Page { return lookPages().Stage(slot, st) }

var lookName = sharedsettings.LookName
