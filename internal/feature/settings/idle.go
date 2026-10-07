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
	return sharedpages.NewIdlePages(sharedpages.IdleOptions{Read: func() sharedpages.IdleState {
		c := config.Get()
		return sharedpages.IdleState{
			Screen:  c.Screen,
			Clock:   c.Clock,
			Idle:    c.Idle,
			Visual:  c.Visual,
			Weather: c.Weather,
		}
	}, Push: shell.Get().Push, Idle: idle.Get(), MediaIdle: media.Get().SetIdle, Label: visuals.Get().SetLabel, Thumbnail: visual.ThumbnailFit, Weather: sharedpages.WeatherActions{Title: weather.Get().Title, Fetch: weather.Get().Fetch, Offered: weather.Get().Offered, SetDashboard: weather.SetDashboard, SetIdle: weather.SetIdle, SetAnimate: weather.SetAnimate, SetThemed: weather.SetThemed, SetPosition: weather.SetPosition, SetAlign: weather.SetAlign, SetSize: weather.SetSize, SetEntity: weather.SetEntity, SetLook: weather.SetLook}})
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
