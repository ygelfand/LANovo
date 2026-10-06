package gui

import (
	"fmt"
	"github.com/ygelfand/libcountertop/pkg/display/style"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

func (a *App) railScreen(v shell.View) *Screen {
	return &Screen{View: v, Build: func(w *gogui.Window) gogui.View { return a.rail(w, v) }}
}

func (a *App) rail(w *gogui.Window, v shell.View) gogui.View {
	t := gogui.CurrentTheme()
	vw, vh := w.WindowSize()
	close := func() { shell.Get().Remove(v) }

	mark := t.TextStyleIconLarge
	mark.Color = t.Cfg.TextStyleDef.Color
	mark.Size = reach() * 0.9
	cell := reach() * 1.5
	var buttons []gogui.View
	for i, e := range drawer.Get().Entries() {
		open := e.Open
		cfg := gogui.ContainerCfg{
			ID:      fmt.Sprintf("rail-%d", i),
			Width:   cell,
			Height:  cell,
			Sizing:  gogui.FixedFixed,
			Padding: gogui.NoPadding,
			HAlign:  gogui.HAlignCenter,
			VAlign:  gogui.VAlignMiddle,
			Radius:  gogui.RadiusMedium,
		}
		st := t.Cfg.TextStyleDef
		if e.Glyph != nil {
			st = mark
		}
		controls().Key(&cfg, &st, style.Rest)
		face := gogui.Label(e.Label(), st)
		if e.Glyph != nil {
			face = gogui.Label(e.Glyph(), st)
		}
		cfg.Content = []gogui.View{face}
		buttons = append(buttons, pressable(gogui.Row, cfg, func(gogui.EventCtx) {
			close()
			open()
		}))
	}

	edge := config.Get().Screen.Drawer
	across := edge == config.EdgeTop || edge == config.EdgeBottom
	strip := panel(gogui.ContainerCfg{
		ID:      "rail",
		Padding: gogui.PaddingSmall,
		Spacing: gogui.SpacingSmall,
		Shadow:  &gogui.BoxShadow{Color: gogui.Black.WithOpacity(0.55), OffsetY: 6, BlurRadius: 24},
		OnClick: func(e gogui.EventCtx) { e.Consume() },
		Content: buttons,
	})
	bar := gogui.Column(strip)
	if across {
		bar = gogui.Row(strip)
	}

	cfg := gogui.ContainerCfg{
		ID:      "rail-away",
		Width:   float32(vw),
		Height:  float32(vh),
		Sizing:  gogui.FixedFixed,
		Padding: gogui.PaddingMedium,
		HAlign:  gogui.HAlignRight,
		VAlign:  gogui.VAlignMiddle,
		Content: []gogui.View{bar},
	}
	switch edge {
	case config.EdgeLeft:
		cfg.HAlign = gogui.HAlignLeft
	case config.EdgeTop:
		cfg.HAlign, cfg.VAlign = gogui.HAlignCenter, gogui.VAlignTop
	case config.EdgeBottom:
		cfg.HAlign, cfg.VAlign = gogui.HAlignCenter, gogui.VAlignBottom
	}
	return pressable(gogui.Column, cfg, func(gogui.EventCtx) { close() })
}
