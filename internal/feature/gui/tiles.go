package gui

import (
	"fmt"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

var lamp = gogui.RGBA(255, 196, 64, 255)

func tabStrip(at ui.Rect, tabs []homecontrol.Selection, open string, pal theme.Theme) gogui.View {
	var pills []gogui.View
	for _, s := range tabs {
		st := gogui.CurrentTheme().Cfg.TextStyleDef
		st.Size *= 1.6
		st.Color = color(pal.Text)
		fill := color(pal.Surface)
		if s.Key == open {
			fill = color(pal.Accent)
			st.Color = color(pal.Background)
		}
		mark := iconStyle(st.Color)
		mark.Size = st.Size
		pills = append(pills, pressable(gogui.Row, gogui.ContainerCfg{
			ID:      "tab-" + s.Key,
			Height:  float32(at.H),
			Sizing:  gogui.FitFixed,
			Color:   fill,
			Radius:  gogui.RadiusLarge,
			Padding: gogui.NewPadding(0, reach()*0.8, 0, reach()*0.6),
			Spacing: gogui.SpacingMedium,
			VAlign:  gogui.VAlignMiddle,
			Content: []gogui.View{gogui.Label(glyphFor(s), mark), gogui.Label(s.Name(), st)},
		}, func(gogui.EventCtx) { homecontrol.Dash().Show(s.Key) }))
	}
	return placed(at, gogui.Row(gogui.ContainerCfg{
		Sizing:  gogui.FillFill,
		Padding: gogui.NoPadding,
		Spacing: gogui.SpacingMedium,
		VAlign:  gogui.VAlignMiddle,
		Content: pills,
	}))
}

func tileBoard(at ui.Rect, s homecontrol.Selection, pal theme.Theme) gogui.View {
	areas, loading := homecontrol.Dash().Tiles(s)
	gap := reach() * 0.25
	cols := max(2, int(float32(at.W)/(reach()*6)))
	side := (float32(at.W) - gap*float32(cols)) / float32(cols)

	var views []gogui.View
	switch {
	case loading:
		views = append(views, gogui.Label(say.T("home.loading"), secondary()))
	case len(areas) == 0:
		views = append(views, gogui.Label(say.T("home.empty"), secondary()))
	}
	var row []gogui.View
	flush := func() {
		if len(row) > 0 {
			views = append(views, gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Spacing: gogui.SpacingPx(gap), Wrap: true, Content: row}))
			row = nil
		}
	}
	for i, a := range areas {
		if !a.Open {
			row = append(row, areaTile(fmt.Sprintf("tile-%d", i), s, a, side, pal))
			continue
		}
		flush()
		views = append(views, openArea(fmt.Sprintf("tile-%d", i), s, a, side, float32(at.W), gap, cols*2, pal))
	}
	flush()

	return placed(at, gogui.Column(gogui.ContainerCfg{
		ID:         "tiles",
		Sizing:     gogui.FillFill,
		Padding:    gogui.NoPadding,
		Spacing:    gogui.SpacingPx(gap),
		Scrollable: true,
		OnGesture:  holdStill,
		Content:    views,
	}))
}

func areaName(a homecontrol.AreaTiles) string {
	if a.ID == "" {
		return say.T("home.noarea")
	}
	return a.Name
}

func areaTile(id string, s homecontrol.Selection, a homecontrol.AreaTiles, side float32, pal theme.Theme) gogui.View {
	return tile(id, side, glyphFor(s), areaName(a), areaStatus(a), a.On > 0, true, pal,
		chevron(id, s, a, pal),
		func(gogui.EventCtx) { homecontrol.Dash().ToggleArea(a) })
}

func areaStatus(a homecontrol.AreaTiles) string {
	switch a.On {
	case 0:
		return say.T("home.tile.all.off")
	case len(a.Tiles):
		return say.T("home.tile.all.on")
	}
	return say.F("home.tile.area", map[string]any{"N": a.On, "Of": len(a.Tiles)})
}

func openArea(id string, s homecontrol.Selection, a homecontrol.AreaTiles, side, width, gap float32, cols int, pal theme.Theme) gogui.View {
	head := areaTile(id+"-area", s, a, side, pal)
	inner := width - 2*gap
	chip := (inner - gap*float32(cols)) / float32(cols)
	var tiles []gogui.View
	for j, t := range a.Tiles {
		tiles = append(tiles, deviceTile(fmt.Sprintf("%s-%d", id, j), s, t, chip, pal))
	}
	return gogui.Column(gogui.ContainerCfg{
		ID:      id,
		Sizing:  gogui.FillFit,
		Padding: gogui.NewPadding(gap, gap, gap, gap),
		Spacing: gogui.SpacingPx(gap),
		Radius:  gogui.RadiusLarge,
		Color:   color(pal.Surface).WithOpacity(0.5),
		Content: []gogui.View{
			head,
			gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Spacing: gogui.SpacingPx(gap), Wrap: true, Content: tiles}),
		},
	})
}

func deviceTile(id string, s homecontrol.Selection, t homecontrol.Tile, side float32, pal theme.Theme) gogui.View {
	status := say.T("home.tile.off")
	switch {
	case !t.Available:
		status = say.T("home.tile.unavailable")
	case t.On && t.Level >= 0:
		status = say.F("home.tile.level", map[string]any{"N": t.Level})
	case t.On:
		status = say.T("home.tile.on")
	}
	return chip(id, side, domainGlyph(t.Domain()), t.Name, status, t.On, t.Available, pal,
		func(gogui.EventCtx) { homecontrol.Dash().ToggleEntity(t) })
}

func chip(id string, side float32, glyph, name, status string, on, available bool, pal theme.Theme, tap func(gogui.EventCtx)) gogui.View {
	t := gogui.CurrentTheme().Cfg
	badgeFill, badgeInk, fill := color(pal.Background).WithOpacity(0.4), color(pal.Text), color(pal.Surface)
	if on {
		badgeFill, badgeInk, fill = lamp, color(theme.Color{R: 40, G: 28, B: 0}), lamp.WithOpacity(0.22)
	}
	mark := iconStyle(badgeInk)
	mark.Size *= 0.7
	small := t.TextStyleDef
	small.Size *= 0.8
	note := secondary()
	note.Size *= 0.75
	badge := gogui.Row(gogui.ContainerCfg{
		Width:   reach() * 0.7,
		Height:  reach() * 0.7,
		Sizing:  gogui.FixedFixed,
		Radius:  gogui.RadiusMedium,
		Color:   badgeFill,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Padding: gogui.NoPadding,
		Content: []gogui.View{gogui.Label(glyph, mark)},
	})
	return pressable(gogui.Row, gogui.ContainerCfg{
		ID:       id,
		Width:    side,
		Sizing:   gogui.FixedFit,
		Padding:  gogui.PaddingSmall,
		Spacing:  gogui.SpacingSmall,
		Radius:   gogui.RadiusMedium,
		Color:    fill,
		VAlign:   gogui.VAlignMiddle,
		Disabled: !available,
		Content: []gogui.View{
			badge,
			gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Clip: true, Content: []gogui.View{
				gogui.Label(name, small),
				gogui.Label(status, note),
			}}),
		},
	}, tap)
}

func glyphFor(s homecontrol.Selection) string {
	if s.Key == "lights" {
		return gogui.IconSunnyO
	}
	return gogui.IconPlug
}

func domainGlyph(domain string) string {
	if domain == "light" {
		return gogui.IconSunnyO
	}
	return gogui.IconPlug
}

func chevron(id string, s homecontrol.Selection, a homecontrol.AreaTiles, pal theme.Theme) gogui.View {
	glyph := gogui.IconArrowDown
	if a.Open {
		glyph = gogui.IconArrowUp
	}
	return pressable(gogui.Row, gogui.ContainerCfg{
		ID:      id + "-chevron",
		Width:   reach(),
		Height:  reach(),
		Sizing:  gogui.FixedFixed,
		Radius:  gogui.RadiusLarge,
		Color:   color(pal.Background).WithOpacity(0.35),
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Padding: gogui.NoPadding,
		Content: []gogui.View{icon(glyph, color(pal.Text))},
	}, func(gogui.EventCtx) { homecontrol.Dash().Expand(s, a.ID) })
}

func tile(id string, side float32, glyph, name, status string, on, available bool, pal theme.Theme, corner gogui.View, tap func(gogui.EventCtx)) gogui.View {
	t := gogui.CurrentTheme().Cfg
	badgeFill, badgeInk, fill := color(pal.Background).WithOpacity(0.4), color(pal.Text), color(pal.Surface)
	if on {
		badgeFill, badgeInk, fill = lamp, color(theme.Color{R: 40, G: 28, B: 0}), lamp.WithOpacity(0.22)
	}
	badge := gogui.Row(gogui.ContainerCfg{
		Width:   reach() * 1.1,
		Height:  reach() * 1.1,
		Sizing:  gogui.FixedFixed,
		Radius:  gogui.RadiusLarge,
		Color:   badgeFill,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Padding: gogui.NoPadding,
		Content: []gogui.View{icon(glyph, badgeInk)},
	})
	top := []gogui.View{badge, gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding})}
	if corner != nil {
		top = append(top, corner)
	}
	title := t.TextStyleDef
	return pressable(gogui.Column, gogui.ContainerCfg{
		ID:       id,
		Width:    side,
		Sizing:   gogui.FixedFit,
		Padding:  gogui.PaddingMedium,
		Spacing:  gogui.SpacingSmall,
		Radius:   gogui.RadiusLarge,
		Color:    fill,
		Disabled: !available,
		Content: []gogui.View{
			gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, VAlign: gogui.VAlignMiddle, Content: top}),
			gogui.Label(name, title),
			gogui.Label(status, secondary()),
		},
	}, tap)
}
