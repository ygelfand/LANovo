package gui

import (
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard/face"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/videoplayer"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/widget"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	sharedwidgets "github.com/ygelfand/libcountertop/pkg/display/widgets"
	sharedhome "github.com/ygelfand/libcountertop/pkg/homeassistant/homecontrol"
	sharedcall "github.com/ygelfand/libcountertop/pkg/media/call"
	"github.com/ygelfand/libcountertop/pkg/say"
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
	case *sharedhome.Home:
		return homeScreen(t)
	case *sharedhome.Picker:
		return pickerScreen(t)
	case *sharedcall.View:
		return callScreen(t)
	case *sharedcall.Profile:
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
	return &Screen{
		Title: p.Title,
		View:  p,
		Build: func(w *gogui.Window) gogui.View { return pageBody(w, p) },
	}
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
	return sharedwidgets.PageBody(w, p, sharedwidgets.PageRenderer{
		Row: presentation.Row, Tile: tileView, Preview: livePreview, Beside: beside,
	})
}

var cameraPreview = &sharedwidgets.LivePreview{}

func livePreview(w *gogui.Window, watch func(ui.Rect) bool) gogui.View {
	return cameraPreview.View(w, say.T("camera.starting"), watch)
}

var beside = sharedwidgets.Beside

func tileView(w *gogui.Window, id, page string, c widget.Cell, tap func(int)) gogui.View {
	t := gogui.CurrentTheme().Cfg
	tw := int(t.SizeTextMedium * 8)
	th := int(float32(tw) * 0.72)
	content := []gogui.View{}
	build, native := clockView(c.Face)
	switch {
	case c.Weather != "":
		content = append(content, weatherSample(w, c.Weather, float32(tw), float32(th)))
	case c.Style != "":
		content = append(content, sharedwidgets.StyleSample(c.Style, float32(tw), float32(th)))
	case c.Palette != nil:
		content = append(content, sharedwidgets.Swatch(*c.Palette, float32(tw), float32(th)))
	case native:
		content = append(content, faceTile(w, build, tw, th))
	case c.Paint != nil:
		content = append(
			content,
			picture(
				painted(
					"tile/"+page+"/"+c.Label+"/"+time.Now().Format("15:04"),
					tw,
					th,
					presentation.Palette().Surface,
					c.Paint,
				),
				tw,
				th,
			),
		)
	}
	return presentation.Toolkit.Tile(id, c.Label, c.Chosen, content, tap)
}

func reach() float32 { return style.Reach() }

var swatchView = sharedwidgets.Swatch

func faceTile(w *gogui.Window, build faceView, tw, th int) gogui.View {
	cfg := config.Get()
	pal := presentation.Palette()
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
