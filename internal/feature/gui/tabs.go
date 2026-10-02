package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type tabKind struct {
	glyph func(dashboard.Tab) string
	board func(ui.Rect, dashboard.Tab, theme.Theme) gogui.View
}

var tabKinds = map[string]tabKind{
	homecontrol.TabKind: {glyph: homeGlyph, board: homeBoard},
}

func homeGlyph(t dashboard.Tab) string {
	return glyphFor(homecontrol.Selection{Key: t.Key})
}

func homeBoard(at ui.Rect, t dashboard.Tab, pal theme.Theme) gogui.View {
	s, ok := homecontrol.Dash().Find(t.Key)
	if !ok {
		return nil
	}
	return tileBoard(at, s, pal)
}

func tabStrip(at ui.Rect, tabs []dashboard.Tab, open string, pal theme.Theme) gogui.View {
	var pills []gogui.View
	for _, t := range tabs {
		st := gogui.CurrentTheme().Cfg.TextStyleDef
		st.Size *= 1.6
		st.Color = color(pal.Text)
		fill := color(pal.Surface)
		if t.Key == open {
			fill = color(pal.Accent)
			st.Color = color(pal.Background)
		}
		content := []gogui.View{gogui.Label(t.Name, st)}
		if kind, ok := tabKinds[t.Kind]; ok && kind.glyph != nil {
			mark := iconStyle(st.Color)
			mark.Size = st.Size
			content = append([]gogui.View{gogui.Label(kind.glyph(t), mark)}, content...)
		}
		pills = append(pills, pressable(gogui.Row, gogui.ContainerCfg{
			ID:      "tab-" + t.Key,
			Height:  float32(at.H),
			Sizing:  gogui.FitFixed,
			Color:   fill,
			Radius:  gogui.RadiusLarge,
			Padding: gogui.NewPadding(0, reach()*0.8, 0, reach()*0.6),
			Spacing: gogui.SpacingMedium,
			VAlign:  gogui.VAlignMiddle,
			Content: content,
		}, func(gogui.EventCtx) { dashboard.Show(t.Key) }))
	}
	return placed(at, gogui.Row(gogui.ContainerCfg{
		Sizing:  gogui.FillFill,
		Padding: gogui.NoPadding,
		Spacing: gogui.SpacingMedium,
		VAlign:  gogui.VAlignMiddle,
		Content: pills,
	}))
}
