package gui

import (
	"image"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

func (a *App) idleScreen(v *idle.View) *Screen {
	return &Screen{
		Clear: true,
		View:  v,
		Build: func(w *gogui.Window) gogui.View { return a.idle(w, v) },
	}
}

func (a *App) idle(w *gogui.Window, v *idle.View) gogui.View {
	vw, vh := w.WindowSize()
	cfg := config.Get()
	pal := palette()
	slots := v.Keep(vw, vh)
	areas := idle.Areas(vw, vh, len(slots))
	box := dashboard.Place(cfg.Idle.Position, cfg.Idle.Align, cfg.Idle.Size, vw, vh)

	var layers []gogui.View
	if len(slots) == 0 {
		layers = append(layers, placed(ui.Rect{W: vw, H: vh}, gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFill, Padding: gogui.NoPadding, Color: color(pal.Background)})))
		if backdrop, behind := poster.Get().Backdrop(vw, vh, image.Rect(box.X, box.Y, box.X+box.W, box.Y+box.H), pal.Background); backdrop != nil {
			layers = append(layers, placed(ui.Rect{W: vw, H: vh}, picture(imageSrc("poster/"+behind, backdrop), vw, vh)))
		}
	}
	if cfg.Screen.Logo {
		layers = append(layers, logo(vw, vh, pal))
	}
	if build, ok := faces[cfg.Idle.Face]; ok {
		r := idle.Reading(cfg, time.Now())
		tones := make([]visual.Traits, len(slots))
		for n, slot := range slots {
			tones[n] = visual.Kind(slot.Kind).Traits()
		}
		for _, p := range idle.Pieces(pal, tones, areas, box) {
			clock := build(w, box, r, cfg.Clock.Ink.Over(p.Palette))
			if p.Clip.W == 0 {
				layers = append(layers, placed(box, clock))
				continue
			}
			inner := ui.Rect{X: box.X - p.Clip.X, Y: box.Y - p.Clip.Y, W: box.W, H: box.H}
			layers = append(layers, placed(p.Clip, gogui.Column(gogui.ContainerCfg{
				Sizing:  gogui.FillFill,
				Padding: gogui.NoPadding,
				Clip:    true,
				Content: []gogui.View{placed(inner, clock)},
			})))
		}
	}
	if wc := cfg.Weather; wc.Idle {
		if v := weatherLayer(w, vw, vh, 0, pal); v != nil {
			layers = append(layers, v)
		}
	}
	return gogui.Column(gogui.ContainerCfg{
		ID:      "idle",
		Sizing:  gogui.FillFill,
		Padding: gogui.NoPadding,
		Content: layers,
		OnClick: func(e gogui.EventCtx) {
			shell.Get().Remove(v)
			e.Consume()
			e.Window.InvalidateLayout()
		},
	})
}

func (a *App) visualScreen(v *visuals.View) *Screen {
	return &Screen{
		Clear: true,
		View:  v,
		Build: func(w *gogui.Window) gogui.View {
			vw, vh := w.WindowSize()
			v.Keep(vw, vh)
			return gogui.Column(gogui.ContainerCfg{
				ID:      "visual",
				Sizing:  gogui.FillFill,
				Padding: gogui.NoPadding,
				OnClick: func(e gogui.EventCtx) {
					shell.Get().Close()
					e.Consume()
					e.Window.InvalidateLayout()
				},
			})
		},
	}
}

func (a *App) seeThrough(on bool) {
	a.mu.Lock()
	r := a.r
	a.mu.Unlock()
	if r == nil {
		return
	}
	if on {
		r.SetClear(&gogui.ColorTransparent)
	} else {
		r.SetClear(nil)
	}
}
