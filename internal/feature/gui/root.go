package gui

import (
	"image"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/assistant"
	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/dashboard/face"
	"github.com/ygelfand/LANovo/internal/feature/poster"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
)

func (a *App) root(w *gogui.Window) gogui.View {
	covering, above := a.nav.Showing(shell.Get().Views())
	if fullLook() || assistant.StagedFull() {
		a.seeThrough(true)
		layers := []gogui.View{gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFill, Padding: gogui.NoPadding})}
		for _, o := range a.nav.Overlays() {
			if v := o.Build(w); v != nil {
				layers = append(layers, floating(o.Priority+1, v))
			}
		}
		return gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFill, Padding: gogui.NoPadding, Content: layers})
	}
	a.seeThrough(covering != nil && covering.Clear)
	base := a.dashboard(w)
	if addr := web.Get().Offering(); addr != "" {
		base = onboard(w, addr)
	}
	switch {
	case covering == nil:
	case covering.Title == "":
		base = covering.Build(w)
	default:
		base = a.page(w, covering)
	}
	layers := []gogui.View{base}
	for _, s := range above {
		layers = append(layers, floating(0, s.Build(w)))
	}
	for _, o := range a.nav.Overlays() {
		if v := o.Build(w); v != nil {
			layers = append(layers, floating(o.Priority+1, v))
		}
	}
	if typing(w) {
		if v := a.keyboard(w); v != nil {
			layers = append(layers, gogui.Column(gogui.ContainerCfg{Float: true, FloatZIndex: 1001, Sizing: gogui.FillFill, Padding: gogui.NoPadding, Content: []gogui.View{v}}))
		}
	}
	return gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFill, Padding: gogui.NoPadding, Content: layers})
}

var floating = widgets.Floating

func (a *App) dashboard(w *gogui.Window) gogui.View {
	vw, vh := w.WindowSize()
	cfg := config.Get()
	pal := palette()
	ink := cfg.Clock.Ink.Over(pal)
	r := face.Read(time.Now(), cfg.Screen.Hours == config.TwentyFourHour)
	if !cfg.Clock.Date {
		r = r.Undated()
	}
	box := dashboard.Box(cfg.Clock.Position, cfg.Clock.Size, vw, vh)
	tabs := dashboard.Tabs()
	showing, tabbed := dashboard.Showing()
	kind, drawn := tabKinds[showing.Kind]
	margin := int(reach() * 0.5)
	strip := ui.Rect{X: margin, Y: margin, W: vw - 2*margin, H: int(reach() * 2)}
	if len(tabs) > 0 && box.Y < strip.Y+strip.H {
		shift := strip.Y + strip.H - box.Y
		box.Y += shift
		box.H = max(box.H-shift, 1)
	}

	var layers []gogui.View
	if backdrop, behind := poster.Get().Backdrop(vw, vh, image.Rect(box.X, box.Y, box.X+box.W, box.Y+box.H), pal.Background); backdrop != nil {
		layers = append(layers, placed(ui.Rect{W: vw, H: vh}, picture(imageSrc("poster/"+behind, backdrop), vw, vh)))
	}

	switch build, ok := clockView(cfg.Clock.Face); {
	case tabbed && drawn:
		below := strip.Y + strip.H + margin
		layers = append(layers, kind.board(ui.Rect{X: margin, Y: below, W: vw - 2*margin, H: vh - below - margin}, showing, pal))
	case ok:
		layers = append(layers, placed(box, build(w, box, r, ink)))
	}
	if wc := cfg.Weather; wc.Dashboard && !(tabbed && drawn) {
		top := float32(0)
		if len(tabs) > 0 {
			top = float32(strip.Y + strip.H)
		}
		if v := weatherLayer(w, vw, vh, top, pal); v != nil {
			layers = append(layers, v)
		}
	}
	if len(tabs) > 0 {
		layers = append(layers, tabStrip(strip, tabs, showing.Key, pal))
	}

	frame := gogui.ContainerCfg{
		ID:      "dashboard",
		Sizing:  gogui.FillFill,
		Padding: gogui.NoPadding,
		Content: layers,
	}
	dash := gogui.Column(frame)
	return gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFill, Padding: gogui.NoPadding, Color: color(pal.Background), Content: []gogui.View{dash}})
}

var placed = widgets.Placed

func (a *App) page(w *gogui.Window, p *Screen) gogui.View {
	return toolkit.PageFrame(w, widgets.FrameOptions{Title: p.Title, Fixed: p.Fixed, Build: p.Build, Back: func() { shell.Get().Pop() }, Gesture: holdStill, Room: editor.Room(w)})
}

func room(w *gogui.Window) []gogui.View { return editor.Room(w) }
