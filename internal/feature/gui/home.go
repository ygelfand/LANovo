package gui

import (
	_ "embed"
	"errors"
	"fmt"
	"github.com/ygelfand/LANovo/internal/feature/settings"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/ui/widget"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	"github.com/ygelfand/libcountertop/pkg/say"
)

//go:embed esphome.svg
var esphomeLogo string

func homeScreen(h *homecontrol.Home) *Screen {
	return &Screen{Title: say.T("home.title"), View: h, Build: func(w *gogui.Window) gogui.View { return homeBody(w) }}
}

func homeBody(w *gogui.Window) gogui.View {
	ha := homeassistant.Get()
	device := map[string]any{"Name": config.Get().Device.Name}
	on := homecontrol.Enabled()
	master := rowView("enabled", "home", widget.Row{Label: say.T("home.enabled"), Kind: widget.Toggle, On: on},
		func(int) { homecontrol.SetEnabled(!on) }, false)
	switch {
	case !on:
		return column([]gogui.View{master})
	case !ha.Connected():
		return column([]gogui.View{master, blocked(w, say.T("home.unconnected"), say.F("home.unconnected.hint", device), nil)})
	case ha.Access() == homeassistant.Unknown:
		return column([]gogui.View{master, blocked(w, say.T("home.checking"), "", nil)})
	case ha.Access() == homeassistant.Refused:
		return column([]gogui.View{master, blocked(w, say.T("home.refused"), say.F("home.refused.hint", device), func(int) { ha.Probe() })})
	}

	views := []gogui.View{master}
	for i, s := range homecontrol.Selections {
		views = append(views, rowView(fmt.Sprintf("sel-%d", i), "home",
			widget.Row{Label: s.Name(), Kind: widget.Chevron, Value: homecontrol.Summary(s.Pick())},
			func(int) { shell.Get().Push(homecontrol.SelectionPage(s)) }, false))
	}
	wc := config.Get().Weather
	says := say.T("weather.off")
	if wc.Entity != "" {
		says = wc.Entity
	}
	views = append(views, rowView("weather", "home",
		widget.Row{Label: say.T("weather.title"), Kind: widget.Chevron, Value: says},
		func(int) { shell.Get().Push(settings.WeatherPage()) }, false))
	views = append(views, separator())
	views = append(views, rowView("refresh", "home",
		widget.Row{Label: say.T("home.refresh")},
		func(int) { homecontrol.Refresh() }, false))
	return column(views)
}

func blocked(w *gogui.Window, title, hint string, retry func(int)) gogui.View {
	t := gogui.CurrentTheme().Cfg
	_, vh := w.WindowSize()
	side := min(t.TextStyleDef.Size*12, float32(vh)*0.2)
	big := t.TextStyleDef
	big.Size *= 1.3
	views := []gogui.View{
		gogui.Svg(gogui.SvgCfg{ID: "esphome", SvgData: esphomeLogo, Width: side, Height: side}),
		gogui.Label(title, big),
	}
	if hint != "" {
		views = append(views, gogui.Text(gogui.TextCfg{Text: hint, TextStyle: secondary(), Mode: gogui.TextModeWrap}))
	}
	if retry != nil {
		views = append(views, rowView("retry", "home", widget.Row{Glyph: gogui.IconSync, Label: say.T("home.check")}, retry, true))
	}
	return gogui.Column(gogui.ContainerCfg{
		Sizing:  gogui.FillFit,
		HAlign:  gogui.HAlignCenter,
		Padding: gogui.PaddingLarge,
		Spacing: gogui.SpacingLarge,
		Content: views,
	})
}

func separator() gogui.View {
	gap := gogui.CurrentTheme().Cfg.TextStyleDef.Size * 0.5
	return gogui.Column(gogui.ContainerCfg{
		Sizing:  gogui.FillFit,
		Padding: gogui.NewPadding(gap, 0, gap, 0),
		Content: []gogui.View{gogui.Row(gogui.ContainerCfg{
			Height: 2,
			Sizing: gogui.FillFixed,
			Color:  gogui.CurrentTheme().Cfg.ColorBorder,
		})},
	})
}

func column(views []gogui.View) gogui.View {
	return gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Spacing: gogui.SpacingTight, Content: views})
}

func pickerScreen(p *homecontrol.Picker) *Screen {
	return &Screen{Title: p.Sel.Name(), View: p, Build: func(*gogui.Window) gogui.View { return pickerBody(p) }}
}

func pickerBody(p *homecontrol.Picker) gogui.View {
	s := p.Snapshot()
	search := textField(gogui.InputCfg{
		ID:            "home-search",
		Text:          s.Query,
		Placeholder:   say.T("home.search"),
		Sizing:        gogui.FillFit,
		OnTextChanged: func(q string, _ gogui.EventCtx) { p.SetQuery(q) },
	}, nil)

	var body []gogui.View
	switch {
	case s.Err != nil:
		body = []gogui.View{
			rowView("failed", "picker", widget.Row{Label: say.T("home.failed"), Hint: failure(s.Err)}, nil, false),
			rowView("retry", "picker", widget.Row{Glyph: gogui.IconSync, Label: say.T("home.retry")}, func(int) { p.Retry() }, true),
		}
		return column(append([]gogui.View{search}, body...))
	case s.Loading:
		return column([]gogui.View{search, rowView("loading", "picker", widget.Row{Label: say.T("home.loading")}, nil, false)})
	}

	var strip []gogui.View
	for _, it := range []struct {
		tab   homecontrol.Tab
		label string
	}{{homecontrol.TabLabels, say.T("home.tab.labels")}, {homecontrol.TabManual, say.T("home.tab.manual")}} {
		st := gogui.CurrentTheme().Cfg.TextStyleDef
		cfg := gogui.ContainerCfg{ID: "home-tab-" + string(it.tab), Padding: gogui.PaddingMedium, VAlign: gogui.VAlignMiddle}
		tab(&cfg, &st, chosen(s.Tab == it.tab))
		cfg.Content = []gogui.View{gogui.Label(it.label, st)}
		pick := it.tab
		strip = append(strip, pressable(gogui.Row, cfg, func(gogui.EventCtx) { p.SetTab(pick) }))
	}
	shown := labelsTab(p, s)
	if s.Tab == homecontrol.TabManual {
		shown = manualTab(p, s)
	}
	tabs := gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Spacing: gogui.SpacingSmall, Content: strip})
	return column([]gogui.View{search, tabs, shown})
}

func failure(err error) string {
	device := map[string]any{"Name": config.Get().Device.Name}
	switch {
	case errors.Is(err, homeassistant.ErrNotAllowed):
		return say.F("home.refused.hint", device)
	case errors.Is(err, homeassistant.ErrNotConnected):
		return say.F("home.unconnected.hint", device)
	}
	return ""
}

func labelsTab(p *homecontrol.Picker, s homecontrol.Snapshot) gogui.View {
	if len(s.Labels) == 0 {
		return column([]gogui.View{rowView("empty", "labels", widget.Row{Label: say.T("home.empty")}, nil, false)})
	}
	var views []gogui.View
	for i, l := range s.Labels {
		views = append(views, rowView(fmt.Sprintf("label-%d", i), "labels", widget.Row{
			Label: l.Name, Hint: say.F("home.label.count", map[string]any{"N": l.Count}),
			Kind: widget.Toggle, On: l.On, Chosen: l.On,
		}, func(int) { p.ToggleLabel(l.ID) }, false))
	}
	return column(views)
}

func manualTab(p *homecontrol.Picker, s homecontrol.Snapshot) gogui.View {
	views := []gogui.View{rowView("all", "manual", widget.Row{
		Label: say.T("home.everything." + p.Sel.Key),
		Kind:  widget.Toggle, On: s.Pick.All, Chosen: s.Pick.All,
	}, func(int) { p.ToggleAll() }, false)}
	if len(s.Areas) == 0 {
		views = append(views, rowView("empty", "manual", widget.Row{Label: say.T("home.empty")}, nil, false))
	}
	for i, a := range s.Areas {
		views = append(views, areaHeader(p, s, i, a))
		if !a.Open {
			continue
		}
		for j, e := range a.Entities {
			name := e.Name
			if name == "" {
				name = e.ID
			}
			row := rowView(fmt.Sprintf("entity-%d-%d", i, j), "manual", widget.Row{
				Label: name, Kind: widget.Toggle, On: e.On, Chosen: e.Included, Dim: !e.Available(),
			}, func(int) { p.ToggleEntity(e.ID) }, false)
			views = append(views, gogui.Row(gogui.ContainerCfg{
				Sizing:  gogui.FillFit,
				Padding: gogui.NewPadding(0, 0, 0, reach()),
				Content: []gogui.View{row},
			}))
		}
	}
	return column(views)
}

func areaHeader(p *homecontrol.Picker, s homecontrol.Snapshot, i int, a homecontrol.AreaRow) gogui.View {
	t := gogui.CurrentTheme().Cfg
	name := a.Name
	if a.ID == "" {
		name = say.T("home.noarea")
	}
	glyph := gogui.IconArrowRight
	if a.Open {
		glyph = gogui.IconArrowDown
	}
	id := fmt.Sprintf("area-%d", i)
	opener := pressable(gogui.Row, gogui.ContainerCfg{
		ID:      id + "-open",
		Sizing:  gogui.FillFit,
		VAlign:  gogui.VAlignMiddle,
		Padding: gogui.NoPadding,
		Spacing: gogui.SpacingMedium,
		Content: []gogui.View{
			icon(glyph, t.ColorTextSecondary),
			gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Content: []gogui.View{
				gogui.Label(name, gogui.TextStyle{}),
				gogui.Label(say.F("home.area.count", map[string]any{"N": a.Included, "Of": len(a.Entities)}), secondary()),
			}}),
		},
	}, func(gogui.EventCtx) { p.Expand(a.ID) })

	content := []gogui.View{opener}
	if a.ID != "" {
		content = append(content, pressable(gogui.Row, gogui.ContainerCfg{
			ID:      id + "-switch",
			Padding: gogui.NoPadding,
			VAlign:  gogui.VAlignMiddle,
			Content: []gogui.View{controls().Toggle(id+"-sw", a.On)},
		}, func(gogui.EventCtx) { p.ToggleArea(a.ID) }))
	}
	cfg := gogui.ContainerCfg{
		ID:      id,
		Sizing:  gogui.FillFit,
		VAlign:  gogui.VAlignMiddle,
		Padding: gogui.PaddingMedium,
		Spacing: gogui.SpacingMedium,
		Radius:  gogui.RadiusMedium,
		Content: content,
	}
	state := style.Rest
	switch {
	case a.On || s.Pick.All:
		state = style.Chosen
	case a.Included > 0:
		state = style.Partial
	}
	listRow(&cfg, state)
	return gogui.Row(cfg)
}
