package settings

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/weather"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/LANovo/internal/ui/visual"
	sharedpages "github.com/ygelfand/libcountertop/pkg/display/settings"
)

type labeled interface {
	comparable
	Label() string
}

func choose[T labeled](
	title string,
	options []T,
	now func() T,
	use func(T),
	preview func(T) func(ui.Surface, ui.Rect, theme.Theme),
) *shell.Page {
	return sharedpages.Choose(title, options, now, use, preview)
}
func idlePages() *sharedpages.IdlePages {
	return sharedpages.NewIdlePages(sharedpages.IdleDependencies{Preferences: preferences(), Shell: shell.Get(), Idle: idle.Get(), Media: media.Get(), Visuals: visuals.Get(), Weather: weather.Get(), Thumbnail: visual.ThumbnailFit})
}

func idlePage() *shell.Page               { return idlePages().Page() }
func idleVisualPage(slot int) *shell.Page { return idlePages().Visual(slot) }
func idleSourcePage(slot int) *shell.Page { return idlePages().Source(slot) }
func WeatherPage() *shell.Page            { return idlePages().Weather() }
func weatherEntityPage() *shell.Page      { return idlePages().WeatherEntities() }
func weatherLookPage() *shell.Page        { return idlePages().WeatherLooks() }

var mediaIndex = sharedpages.MediaIndex
var mediaLevel = sharedpages.MediaLevel
var mediaSnap = sharedpages.MediaSnap
var noneTile = sharedpages.NoneTile

func slotVisual(slot int) config.IdleVisual { return idlePages().SlotVisual(slot) }
func idleFacePreview(f config.Face) func(ui.Surface, ui.Rect, theme.Theme) {
	return idlePages().FacePreview(f)
}
func idlePlaced(change func(*config.Idle)) func(ui.Surface, ui.Rect, theme.Theme) {
	return idlePages().Placed(change)
}
