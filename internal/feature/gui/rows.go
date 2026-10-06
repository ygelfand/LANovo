package gui

import (
	"fmt"
	"slices"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/dashboard/face"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/videoplayer"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/LANovo/internal/ui/widget"
	"github.com/ygelfand/libcountertop/pkg/display/style"
)

func (a *App) screenFor(v shell.View) *Screen {
	switch t := v.(type) {
	case *idle.View:
		return a.idleScreen(t)
	case *visuals.View:
		return a.visualScreen(t)
	case *videoplayer.Page:
		return a.videoScreen(t)
	case *shell.Page:
		return pageScreen(t)
	case *homecontrol.Home:
		return homeScreen(t)
	case *homecontrol.Picker:
		return pickerScreen(t)
	case *call.View:
		return callScreen(t)
	case *call.Profile:
		return profileScreen(t)
	}
	switch v {
	case media.Page():
		return playerScreen(v)
	case volume.Get().Card():
		return volumeScreen(v)
	case drawer.Get():
		return a.railScreen(v)
	}
	return nil
}

func pageScreen(p *shell.Page) *Screen {
	return &Screen{Title: p.Title, View: p, Build: func(w *gogui.Window) gogui.View { return pageBody(w, p) }}
}

func iconStyle(c gogui.Color) gogui.TextStyle {
	st := gogui.CurrentTheme().TextStyleIconMedium
	st.Color = c
	return st
}

func icon(glyph string, c gogui.Color) gogui.View { return gogui.Label(glyph, iconStyle(c)) }

func secondary() gogui.TextStyle {
	t := gogui.CurrentTheme().Cfg
	st := t.TextStyleDef
	st.Color = t.ColorTextSecondary
	return st
}

func pageBody(w *gogui.Window, p *shell.Page) gogui.View {
	var views []gogui.View
	switch {
	case p.Build != nil:
		rows, taps := p.Build()
		glyphs := slices.ContainsFunc(rows, func(r widget.Row) bool { return r.Glyph != "" })
		for i, r := range rows {
			var tap func(int)
			if i < len(taps) {
				tap = taps[i]
			}
			views = append(views, rowView(fmt.Sprintf("row-%d", i), p.Title, r, tap, glyphs))
		}
	case p.Tiles != nil:
		cells, taps := p.Tiles()
		var tiles []gogui.View
		for i, c := range cells {
			var tap func(int)
			if i < len(taps) {
				tap = taps[i]
			}
			tiles = append(tiles, tileView(w, fmt.Sprintf("tile-%d", i), p.Title, c, tap))
		}
		views = append(views, gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Wrap: true, Spacing: gogui.SpacingMedium, Content: tiles}))
	}
	list := gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Spacing: gogui.SpacingTight, Content: views})
	if p.Preview == nil {
		return list
	}
	return beside(w, livePreview(w, p.Preview), list)
}

var (
	drafts = map[string]string{}
	keyTop float32
)

func clear(w *gogui.Window, bottom float32) {
	if keyTop <= 0 || bottom <= keyTop-reach()*0.3 {
		return
	}
	by := bottom - keyTop + reach()*0.5
	w.QueueCommand(func(w *gogui.Window) {
		w.ScrollVerticalTo("page", w.ScrollVerticalOffset("page")-by)
	})
}

func fieldRow(id string, r widget.Row) gogui.View {
	fid := id + "-field"
	text, ok := drafts[fid]
	if !ok {
		text = r.Value
	}
	save := r.Save
	return textField(gogui.InputCfg{
		ID:     fid,
		Text:   text,
		Sizing: gogui.FillFit,
		OnTextChanged: func(s string, ev gogui.EventCtx) {
			drafts[fid] = s
			ev.Window.InvalidateLayout()
		},
		OnTextCommit: func(_ string, _ gogui.InputCommitReason, ev gogui.EventCtx) {
			delete(drafts, fid)
			ev.Window.InvalidateLayout()
		},
	}, func() {
		if s, ok := drafts[fid]; ok && save != nil {
			save(s)
		}
	})
}

var previewing bool

func livePreview(w *gogui.Window, watch func(ui.Rect) bool) gogui.View {
	t := gogui.CurrentTheme()
	vw, vh := w.WindowSize()
	bw, bh, sizing := float32(vw)*0.45, float32(vh)*0.6, gogui.FixedFixed
	if vh > vw {
		bw, bh, sizing = 0, float32(vh)*0.3, gogui.FillFixed
	}
	var content []gogui.View
	fill := gogui.Color{}
	if !previewing {
		content = append(content, gogui.Label(say.T("camera.starting"), secondary()))
		fill = t.Cfg.ColorPanel
	}
	return gogui.Column(gogui.ContainerCfg{
		ID:      "preview",
		Width:   bw,
		Height:  bh,
		Sizing:  sizing,
		Color:   fill,
		Radius:  gogui.RadiusMedium,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Content: content,
		AmendLayout: func(e gogui.EventCtx) {
			s := e.Layout.Shape
			previewing = watch(ui.Rect{X: int(s.X), Y: int(s.Y), W: int(s.Width), H: int(s.Height)})
		},
	})
}

func beside(w *gogui.Window, aside, list gogui.View) gogui.View {
	vw, vh := w.WindowSize()
	cfg := gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Spacing: gogui.SpacingLarge, Content: []gogui.View{aside, list}}
	if vh > vw {
		return gogui.Column(cfg)
	}
	return gogui.Row(cfg)
}

func tileView(w *gogui.Window, id, page string, c widget.Cell, tap func(int)) gogui.View {
	t := gogui.CurrentTheme().Cfg
	tw := int(t.SizeTextMedium * 8)
	th := int(float32(tw) * 0.72)
	content := []gogui.View{}
	build, native := faces[c.Face]
	switch {
	case c.Weather != "":
		content = append(content, weatherSample(w, c.Weather, float32(tw), float32(th)))
	case c.Style != "":
		content = append(content, styleSample(c.Style, float32(tw), float32(th)))
	case c.Palette != nil:
		content = append(content, swatchView(*c.Palette, float32(tw), float32(th)))
	case native:
		content = append(content, faceTile(w, build, tw, th))
	case c.Paint != nil:
		content = append(content, picture(painted("tile/"+page+"/"+c.Label+"/"+time.Now().Format("15:04"), tw, th, palette().Surface, c.Paint), tw, th))
	}
	content = append(content, gogui.Label(c.Label, gogui.TextStyle{}))
	cfg := gogui.ContainerCfg{
		ID:         id,
		HAlign:     gogui.HAlignCenter,
		Padding:    gogui.PaddingSmall,
		Spacing:    gogui.SpacingSmall,
		Radius:     gogui.RadiusMedium,
		SizeBorder: gogui.BorderPx(3),
		Clip:       true,
		Content:    content,
	}
	if c.Chosen {
		cfg.ColorBorder = t.ColorAccent
	}
	controls().Tile(&cfg, chosen(c.Chosen))
	return pressable(gogui.Column, cfg, tapped(tap, 0))
}

func preview(page string, r widget.Row) gogui.View {
	h := int(gogui.CurrentTheme().Cfg.SizeTextMedium * 2.4)
	w := h * 2
	return picture(painted("row/"+page+"/"+r.Label+"/"+r.Value+"/"+time.Now().Format("15:04"), w, h, palette().Background, r.Preview), w, h)
}

func rowView(id, page string, r widget.Row, tap func(int), glyphs bool) gogui.View {
	label := []gogui.View{gogui.Label(r.Label, gogui.TextStyle{})}
	if r.Hint != "" {
		label = append(label, gogui.Label(r.Hint, secondary()))
	}
	head := gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Content: label})

	content := []gogui.View{head}
	if glyphs {
		var mark []gogui.View
		if r.Glyph != "" {
			mark = []gogui.View{icon(r.Glyph, gogui.CurrentTheme().Cfg.ColorTextSecondary)}
		}
		content = []gogui.View{gogui.Row(gogui.ContainerCfg{
			Width:   reach(),
			Sizing:  gogui.FixedFit,
			Padding: gogui.NoPadding,
			HAlign:  gogui.HAlignCenter,
			Content: mark,
		}), head}
	}
	var onClick func(gogui.EventCtx)
	switch r.Kind {
	case widget.Toggle:
		content = append(content, controls().Toggle(id+"-switch", r.On))
		onClick = tapped(tap, 0)
	case widget.Slider:
		snap := r.Snap
		says := r.Value
		if says == "" {
			says = fmt.Sprintf("%d%%", r.Level)
		}
		content = append(content, gogui.Row(gogui.ContainerCfg{
			Padding: gogui.NewPadding(0, reach()*0.4, 0, 0),
			Content: []gogui.View{gogui.Label(says, secondary())},
		}))
		level := gogui.SliderCfg{
			ID: id + "-slider", Value: float32(r.Level), Min: 0, Max: 100, Sizing: gogui.FillFit, Height: reach(),
			OnChange: func(v float32, e gogui.EventCtx) {
				level := int(v + 0.5)
				if snap != nil {
					level = snap(level)
				}
				if tap != nil {
					tap(level)
				}
				e.Window.InvalidateLayout()
			},
		}
		content = append(content, grip(slider(level)))
	case widget.Field:
		content = append(content, fieldRow(id, r))
	case widget.Chevron:
		if r.Value != "" {
			content = append(content, gogui.Label(r.Value, secondary()))
		}
		content = append(content, icon(gogui.IconArrowRight, gogui.CurrentTheme().Cfg.ColorTextSecondary))
		onClick = tapped(tap, 0)
	default:
		if r.Value != "" {
			content = append(content, gogui.Label(r.Value, secondary()))
		}
		if r.Preview != nil {
			content = append(content, preview(page, r))
		}
		if r.Chosen {
			content = append(content, icon(gogui.IconCheck, gogui.CurrentTheme().Cfg.ColorAccent))
		}
		onClick = tapped(tap, 0)
	}
	cfg := gogui.ContainerCfg{
		ID:       id,
		Sizing:   gogui.FillFit,
		VAlign:   gogui.VAlignMiddle,
		Padding:  gogui.PaddingMedium,
		Spacing:  gogui.SpacingMedium,
		Radius:   gogui.RadiusMedium,
		Disabled: r.Dim,
		Content:  content,
	}
	listRow(&cfg, chosen(r.Kind == widget.Toggle && r.Chosen))
	return pressable(gogui.Row, cfg, onClick)
}

func outline(cfg *gogui.ContainerCfg) {
	cfg.ColorBorder = gogui.CurrentTheme().Cfg.ColorAccent.WithOpacity(0.7)
	cfg.SizeBorder = gogui.BorderPx(2)
}

func highlight(cfg *gogui.ContainerCfg) {
	accent := gogui.CurrentTheme().Cfg.ColorAccent
	cfg.Color = accent.WithOpacity(0.18)
	cfg.ColorBorder = accent
	cfg.SizeBorder = gogui.BorderPx(2)
}

var gripped bool

func panning(e gogui.EventCtx) (gogui.GesturePhase, bool) {
	if e.Event == nil || e.Event.GestureType != gogui.GesturePan {
		return 0, false
	}
	return e.Event.GesturePhase, true
}

func reach() float32 { return style.Reach() }

func grip(v gogui.View) gogui.View {
	return gogui.Row(gogui.ContainerCfg{
		Sizing:  gogui.FillFit,
		Padding: gogui.NoPadding,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Content: []gogui.View{v},
		OnGesture: func(e gogui.EventCtx) {
			if phase, ok := panning(e); ok {
				gripped = phase == gogui.GesturePhaseBegan || phase == gogui.GesturePhaseChanged
				e.Consume()
			}
		},
	})
}

func holdStill(e gogui.EventCtx) {
	phase, ok := panning(e)
	if ok {
		release(e)
	}
	if !ok || !gripped {
		return
	}
	if phase != gogui.GesturePhaseBegan && phase != gogui.GesturePhaseChanged {
		gripped = false
	}
	e.Consume()
}

func swatchView(p theme.Theme, w, h float32) gogui.View {
	side := min(h*0.28, (w*0.8)/5)
	var chips []gogui.View
	for _, c := range []theme.Color{p.Surface, p.Text, p.Accent, p.Accent2} {
		chips = append(chips, gogui.Column(gogui.ContainerCfg{Width: side, Height: side, Sizing: gogui.FixedFixed, Color: color(c), Radius: gogui.RadiusPx(side * 0.2), Padding: gogui.NoPadding}))
	}
	return gogui.Row(gogui.ContainerCfg{
		Width:       w,
		Height:      h,
		Sizing:      gogui.FixedFixed,
		Color:       color(p.Background),
		ColorBorder: color(p.Text.Blend(p.Background, 0.7)),
		SizeBorder:  gogui.BorderPx(1),
		Radius:      gogui.RadiusMedium,
		HAlign:      gogui.HAlignCenter,
		VAlign:      gogui.VAlignMiddle,
		Spacing:     gogui.SpacingPx(side * 0.35),
		Padding:     gogui.NoPadding,
		Content:     chips,
	})
}

func faceTile(w *gogui.Window, build faceView, tw, th int) gogui.View {
	cfg := config.Get()
	pal := palette()
	r := face.Read(time.Now(), cfg.Screen.Hours == config.TwentyFourHour).Undated()
	return gogui.Column(gogui.ContainerCfg{
		Width:   float32(tw),
		Height:  float32(th),
		Sizing:  gogui.FixedFixed,
		Color:   color(pal.Surface),
		Radius:  gogui.RadiusMedium,
		Padding: gogui.NoPadding,
		Clip:    true,
		Content: []gogui.View{build(w, ui.Rect{W: tw, H: th}, r, cfg.Clock.Ink.Over(pal))},
	})
}
