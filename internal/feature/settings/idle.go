package settings

import (
	"slices"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/dashboard/face"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/weather"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/LANovo/internal/ui/visual"
	"github.com/ygelfand/LANovo/internal/ui/widget"
)

type labeled interface {
	comparable
	Label() string
}

func choose[T labeled](title string, options []T, now func() T, use func(T), preview func(T) func(ui.Surface, ui.Rect, theme.Theme)) *shell.Page {
	return &shell.Page{
		Title: title,
		Build: func() ([]widget.Row, []func(int)) {
			chosen := now()
			rows := make([]widget.Row, 0, len(options))
			acts := make([]func(int), 0, len(options))
			for _, o := range options {
				row := widget.Row{Label: o.Label(), Chosen: o == chosen}
				if preview != nil {
					row.Preview = preview(o)
				}
				rows = append(rows, row)
				acts = append(acts, func(int) { use(o) })
			}
			return rows, acts
		},
	}
}

func idlePage() *shell.Page {
	return &shell.Page{
		Title: say.T("idle.title"),
		Build: func() ([]widget.Row, []func(int)) {
			c := config.Get().Idle
			i := idle.Get()
			return []widget.Row{
					{Label: say.T("idle.after"), Kind: widget.Chevron, Value: c.After.Label()},
					{Label: say.T("idle.media"), Hint: say.T("idle.media.hint"), Kind: widget.Slider, Level: mediaLevel(c.Media), Snap: mediaSnap, Value: c.Media.Label()},
					{Label: say.T("idle.visual1"), Kind: widget.Chevron, Value: idle.KindLabel(c.First.Kind)},
					{Label: say.T("idle.source1"), Kind: widget.Chevron, Value: c.First.Source.Label()},
					{Label: say.T("idle.visual2"), Kind: widget.Chevron, Value: idle.KindLabel(c.Second.Kind)},
					{Label: say.T("idle.source2"), Kind: widget.Chevron, Value: c.Second.Source.Label()},
					{Label: say.T("idle.face"), Kind: widget.Chevron, Value: c.Face.Label()},
					{Label: say.T("idle.vertical"), Kind: widget.Chevron, Value: c.Position.Label()},
					{Label: say.T("idle.horizontal"), Kind: widget.Chevron, Value: c.Align.Label()},
					{Label: say.T("idle.size"), Kind: widget.Chevron, Value: c.Size.Label()},
					{Label: say.T("idle.label"), Kind: widget.Field, Value: config.Get().Visual.Label, Save: visuals.Get().SetLabel},
				}, []func(int){
					open(choose(say.T("idle.after.title"), config.Delays(),
						func() config.Delay { return config.Get().Idle.After }, i.SetAfter, nil)),
					func(level int) { media.Get().SetIdle(config.MediaDelays()[mediaIndex(level)]) },
					open(idleVisualPage(0)),
					open(idleSourcePage(0)),
					open(idleVisualPage(1)),
					open(idleSourcePage(1)),
					open(choose(say.T("idle.face"), config.IdleFaces(),
						func() config.Face { return config.Get().Idle.Face }, i.SetFace, idleFacePreview)),
					open(choose(say.T("idle.vertical"), config.Positions(),
						func() config.Position { return config.Get().Idle.Position }, i.SetPosition,
						func(p config.Position) func(ui.Surface, ui.Rect, theme.Theme) {
							return idlePlaced(func(c *config.Idle) { c.Position = p })
						})),
					open(choose(say.T("idle.horizontal"), config.Aligns(),
						func() config.Align { return config.Get().Idle.Align }, i.SetAlign,
						func(a config.Align) func(ui.Surface, ui.Rect, theme.Theme) {
							return idlePlaced(func(c *config.Idle) { c.Align = a })
						})),
					open(choose(say.T("idle.size"), config.Sizes(),
						func() config.Size { return config.Get().Idle.Size }, i.SetSize,
						func(s config.Size) func(ui.Surface, ui.Rect, theme.Theme) {
							return idlePlaced(func(c *config.Idle) { c.Size = s })
						})),
					nil,
				}
		},
	}
}

func mediaIndex(level int) int {
	n := len(config.MediaDelays()) - 1
	return min(max((level*n+50)/100, 0), n)
}

func mediaLevel(d config.Delay) int {
	i := max(slices.Index(config.MediaDelays(), d), 0)
	return i * 100 / (len(config.MediaDelays()) - 1)
}

func mediaSnap(level int) int { return mediaLevel(config.MediaDelays()[mediaIndex(level)]) }

func slotVisual(slot int) config.IdleVisual {
	c := config.Get().Idle
	if slot == 0 {
		return c.First
	}
	return c.Second
}

func idleVisualPage(slot int) *shell.Page {
	title := say.T("idle.visual1")
	if slot == 1 {
		title = say.T("idle.visual2")
	}
	return visualPicker(title, say.T("idle.none"),
		func() { idle.Get().SetKind(slot, "") },
		func() string { return slotVisual(slot).Kind },
		func(k visual.Kind) { idle.Get().SetKind(slot, string(k)) })
}

func noneTile(s ui.Surface, at ui.Rect, palette theme.Theme) {
	ui.FillRect(s, at, palette.Surface)
}

func idleSourcePage(slot int) *shell.Page {
	title := say.T("idle.source1")
	if slot == 1 {
		title = say.T("idle.source2")
	}
	return choose(title, config.Sources(),
		func() config.Source { return slotVisual(slot).Source },
		func(s config.Source) { idle.Get().SetSource(slot, s) }, nil)
}

func idleFacePreview(f config.Face) func(ui.Surface, ui.Rect, theme.Theme) {
	return func(s ui.Surface, at ui.Rect, palette theme.Theme) {
		if f == config.FaceNone {
			return
		}
		cfg := config.Get()
		r := face.Read(time.Now(), cfg.Screen.Hours == config.TwentyFourHour)
		face.Of(f).Draw(s, at, r.Undated(), cfg.Clock.Ink.Over(palette))
	}
}

func idlePlaced(change func(*config.Idle)) func(ui.Surface, ui.Rect, theme.Theme) {
	return func(s ui.Surface, box ui.Rect, palette theme.Theme) {
		cfg := config.Get()
		c := cfg.Idle
		change(&c)
		if c.Face == config.FaceNone {
			return
		}
		r := face.Read(time.Now(), cfg.Screen.Hours == config.TwentyFourHour)
		within := dashboard.Place(c.Position, c.Align, c.Size, box.W, box.H)
		within.X += box.X
		within.Y += box.Y
		face.Of(c.Face).Draw(s, within, r.Undated(), cfg.Clock.Ink.Over(palette))
	}
}

func WeatherPage() *shell.Page {
	return &shell.Page{
		Title: say.T("weather.title"),
		Build: func() ([]widget.Row, []func(int)) {
			c := config.Get().Weather
			return []widget.Row{
					{Label: say.T("weather.entity"), Kind: widget.Chevron, Value: weather.Get().Title(c.Entity)},
					{Label: say.T("weather.dashboard"), Kind: widget.Toggle, On: c.Dashboard},
					{Label: say.T("weather.idle"), Kind: widget.Toggle, On: c.Idle},
					{Label: say.T("weather.look"), Kind: widget.Chevron, Value: c.Look.Label()},
					{Label: say.T("weather.animate"), Kind: widget.Toggle, On: c.Animate},
					{Label: say.T("weather.themed"), Hint: say.T("weather.themed.hint"), Kind: widget.Toggle, On: c.Themed},
					{Label: say.T("idle.vertical"), Kind: widget.Chevron, Value: c.Position.Label()},
					{Label: say.T("idle.horizontal"), Kind: widget.Chevron, Value: c.Align.Label()},
					{Label: say.T("weather.size"), Kind: widget.Chevron, Value: c.Size.Label()},
				}, []func(int){
					func(int) {
						weather.Get().Fetch()
						shell.Get().Push(weatherEntityPage())
					},
					func(int) { weather.SetDashboard(!c.Dashboard) },
					func(int) { weather.SetIdle(!c.Idle) },
					open(weatherLookPage()),
					func(int) { weather.SetAnimate(!c.Animate) },
					func(int) { weather.SetThemed(!c.Themed) },
					open(choose(say.T("idle.vertical"), config.Positions(),
						func() config.Position { return config.Get().Weather.Position }, weather.SetPosition, nil)),
					open(choose(say.T("idle.horizontal"), config.Aligns(),
						func() config.Align { return config.Get().Weather.Align }, weather.SetAlign, nil)),
					open(choose(say.T("weather.size"), config.Sizes(),
						func() config.Size { return config.Get().Weather.Size }, weather.SetSize, nil)),
				}
		},
	}
}

func weatherEntityPage() *shell.Page {
	return &shell.Page{
		Title: say.T("weather.entity"),
		Build: func() ([]widget.Row, []func(int)) {
			list, err, fetched := weather.Get().Offered()
			switch {
			case !fetched:
				return []widget.Row{{Label: say.T("home.loading")}}, []func(int){nil}
			case err != nil:
				return []widget.Row{{Label: say.T("home.failed"), Hint: err.Error()}, {Label: say.T("home.retry")}},
					[]func(int){nil, func(int) { weather.Get().Fetch() }}
			case len(list) == 0:
				return []widget.Row{{Label: say.T("weather.none")}}, []func(int){nil}
			}
			now := config.Get().Weather.Entity
			rows := make([]widget.Row, 0, len(list))
			taps := make([]func(int), 0, len(list))
			for _, e := range list {
				rows = append(rows, widget.Row{Label: e.Name, Hint: e.ID, Chosen: e.ID == now})
				taps = append(taps, func(int) { weather.SetEntity(e.ID) })
			}
			return rows, taps
		},
	}
}

func weatherLookPage() *shell.Page {
	return &shell.Page{
		Title: say.T("weather.look"),
		Tiles: func() ([]widget.Cell, []func(int)) {
			now := config.Get().Weather.Look
			var cells []widget.Cell
			var taps []func(int)
			for _, l := range config.WeatherLooks() {
				cells = append(cells, widget.Cell{Label: l.Label(), Chosen: l == now, Weather: l})
				taps = append(taps, func(int) { weather.SetLook(l) })
			}
			return cells, taps
		},
	}
}
