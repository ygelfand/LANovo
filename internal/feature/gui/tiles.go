package gui

import (
	"fmt"
	"math"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	"github.com/ygelfand/libcountertop/pkg/say"
)

var lamp = gogui.RGBA(255, 196, 64, 255)

func tileBoard(at ui.Rect, s homecontrol.Selection, pal theme.Theme) gogui.View {
	areas, loading := homecontrol.Dash().Tiles(s)
	gap := reach() * 0.25
	cols := max(2, int(float32(at.W)/(reach()*6)))
	side := cell(float32(at.W)-gap, gap, cols)

	var views []gogui.View
	switch {
	case loading:
		views = append(views, gogui.Label(say.T("home.loading"), secondary()))
	case len(areas) == 0:
		views = append(views, gogui.Label(say.T("home.empty"), secondary()))
	}
	stacks := make([][]gogui.View, cols)
	for i, a := range areas {
		stacks[i%cols] = append(stacks[i%cols], card(fmt.Sprintf("tile-%d", i), s, a, side, pal))
	}
	var columns []gogui.View
	for _, stack := range stacks {
		columns = append(
			columns,
			gogui.Column(
				gogui.ContainerCfg{
					Width:   side,
					Sizing:  gogui.FixedFit,
					Padding: gogui.NoPadding,
					Spacing: gogui.SpacingPx(gap),
					Content: stack,
				},
			),
		)
	}
	if len(areas) > 0 {
		views = append(
			views,
			gogui.Row(
				gogui.ContainerCfg{
					Sizing:  gogui.FillFit,
					Padding: gogui.NoPadding,
					Spacing: gogui.SpacingPx(gap),
					Content: columns,
				},
			),
		)
	}

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

func card(
	id string,
	s homecontrol.Selection,
	a homecontrol.AreaTiles,
	side float32,
	pal theme.Theme,
) gogui.View {
	if a.Solo {
		t := a.Tiles[0]
		return tile(
			id,
			side,
			domainGlyph(t.Domain()),
			t.Name,
			deviceStatus(t),
			t.On,
			t.Available,
			pal,
			nil,
			nil,
			func(gogui.EventCtx) { homecontrol.Dash().ToggleEntity(t) },
		)
	}
	var chips []gogui.View
	if a.Open {
		for j, t := range a.Tiles {
			chips = append(
				chips,
				chip(
					fmt.Sprintf("%s-%d", id, j),
					domainGlyph(t.Domain()),
					t.Name,
					deviceStatus(t),
					t.On,
					t.Available,
					pal,
					func(gogui.EventCtx) { homecontrol.Dash().ToggleEntity(t) },
				),
			)
		}
	}
	return tile(id, side, glyphFor(s), areaName(a), areaStatus(a), a.On > 0, true, pal,
		chevron(id, s, a, pal), chips,
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

func cell(width, gap float32, n int) float32 {
	return float32(math.Floor(float64((width - gap*float32(n-1)) / float32(n))))
}

func tint(on bool, pal theme.Theme) (badgeFill, badgeInk, fill gogui.Color) {
	if on {
		return lamp, color(theme.Color{R: 40, G: 28, B: 0}), lamp.WithOpacity(0.22)
	}
	return color(pal.Background).WithOpacity(0.4), color(pal.Text), color(pal.Surface)
}

func deviceStatus(t homecontrol.Tile) string {
	switch {
	case !t.Available:
		return say.T("home.tile.unavailable")
	case t.On && t.Level >= 0:
		return say.F("home.tile.level", map[string]any{"N": t.Level})
	case t.On:
		return say.T("home.tile.on")
	}
	return say.T("home.tile.off")
}

func chip(
	id string,
	glyph, name, status string,
	on, available bool,
	pal theme.Theme,
	tap func(gogui.EventCtx),
) gogui.View {
	t := gogui.CurrentTheme().Cfg
	badgeFill, badgeInk, fill := tint(on, pal)
	if !on {
		fill = color(pal.Background).WithOpacity(0.35)
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
	cfg := gogui.ContainerCfg{
		ID:       id,
		Sizing:   gogui.FillFit,
		Padding:  gogui.PaddingSmall,
		Spacing:  gogui.SpacingSmall,
		Radius:   gogui.RadiusMedium,
		Color:    fill,
		VAlign:   gogui.VAlignMiddle,
		Disabled: !available,
		Content: []gogui.View{
			badge,
			gogui.Column(
				gogui.ContainerCfg{
					Sizing:  gogui.FillFit,
					Padding: gogui.NoPadding,
					Clip:    true,
					Content: []gogui.View{
						gogui.Label(name, small),
						gogui.Label(status, note),
					},
				},
			),
		},
	}
	controls().Tile(&cfg, chosen(on))
	return pressable(gogui.Row, cfg, tap)
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

func chevron(
	id string,
	s homecontrol.Selection,
	a homecontrol.AreaTiles,
	pal theme.Theme,
) gogui.View {
	glyph := gogui.IconArrowDown
	if a.Open {
		glyph = gogui.IconArrowUp
	}
	return iconKey(gogui.ContainerCfg{
		ID:      id + "-chevron",
		Width:   reach(),
		Height:  reach(),
		Sizing:  gogui.FixedFixed,
		Radius:  gogui.RadiusLarge,
		Color:   color(pal.Background).WithOpacity(0.35),
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Padding: gogui.NoPadding,
	}, glyph, iconStyle(color(pal.Text)), style.Partial, func(gogui.EventCtx) { homecontrol.Dash().Expand(s, a.ID) })
}

func tile(
	id string,
	side float32,
	glyph, name, status string,
	on, available bool,
	pal theme.Theme,
	corner gogui.View,
	more []gogui.View,
	tap func(gogui.EventCtx),
) gogui.View {
	t := gogui.CurrentTheme().Cfg
	badgeFill, badgeInk, fill := tint(on, pal)
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
	top := []gogui.View{
		badge,
		gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding}),
	}
	if corner != nil {
		top = append(top, corner)
	}
	title := t.TextStyleDef
	cfg := gogui.ContainerCfg{
		ID:       id,
		Width:    side,
		Sizing:   gogui.FixedFit,
		Padding:  gogui.PaddingMedium,
		Spacing:  gogui.SpacingSmall,
		Radius:   gogui.RadiusLarge,
		Color:    fill,
		Disabled: !available,
		Content: append([]gogui.View{
			gogui.Row(
				gogui.ContainerCfg{
					Sizing:  gogui.FillFit,
					Padding: gogui.NoPadding,
					VAlign:  gogui.VAlignMiddle,
					Content: top,
				},
			),
			gogui.Label(name, title),
			gogui.Label(status, secondary()),
		}, more...),
	}
	controls().Tile(&cfg, chosen(on))
	return pressable(gogui.Column, cfg, tap)
}
