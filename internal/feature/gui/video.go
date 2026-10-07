package gui

import (
	"fmt"
	"math"
	"strings"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/videoplayer"
	"github.com/ygelfand/libcountertop/pkg/say"
)

const (
	videoSkip  = 10 * time.Second
	spinDots   = 12
	spinStep   = 100 * time.Millisecond
	scrimAlpha = 170
)

var (
	videoInk   = gogui.Color{R: 0xf2, G: 0xf2, B: 0xf2, A: 0xff}
	videoQuiet = gogui.Color{R: 0xb0, G: 0xb0, B: 0xb0, A: 0xff}
	videoScrim = gogui.Color{A: scrimAlpha}
)

func (a *App) videoScreen(p *videoplayer.Page) *Screen {
	return &Screen{
		Clear: true,
		View:  p,
		Build: func(w *gogui.Window) gogui.View { return a.video(w, p) },
	}
}

func (a *App) video(w *gogui.Window, p *videoplayer.Page) gogui.View {
	vw, vh := w.WindowSize()
	look := p.Look()
	full := gogui.ContainerCfg{Sizing: gogui.FillFill, Padding: gogui.NoPadding}

	var layers []gogui.View
	if look.Picture {
		if vh > vw && look.FrameW > 0 && look.FrameH > 0 {
			if mark := look.Controls.Now().Mark; mark != nil {
				above := (vh - vw*look.FrameH/look.FrameW) / 2
				layers = append(layers, floatAt(0, 0, float32(vw), float32(above), centredImage(still(mark), float32(vw)/2, float32(above)/2)))
			}
		}
	} else {
		black := full
		black.Color = gogui.Color{A: 0xff}
		layers = append(layers, floatAt(0, 0, float32(vw), float32(vh), gogui.Column(black)))
		layers = append(layers, floatAt(0, 0, float32(vw), float32(vh), backdrop(look, vw, vh)))
	}
	if look.Spinning {
		layers = append(layers, floatAt(0, 0, float32(vw), float32(vh), spinner(vw, vh, time.Since(look.Began))))
	}
	if look.Shown {
		layers = append(layers, floatAt(0, 0, float32(vw), 0, a.videoHeader(p)))
		layers = append(layers, gogui.Column(gogui.ContainerCfg{
			Float:       true,
			FloatAnchor: gogui.FloatBottomLeft,
			FloatTieOff: gogui.FloatBottomLeft,
			Width:       float32(vw),
			Sizing:      gogui.FixedFit,
			Padding:     gogui.NoPadding,
			Content:     []gogui.View{a.videoBand(w, p, look)},
		}))
	}

	cfg := full
	cfg.ID = "video"
	cfg.Content = layers
	cfg.OnClick = func(e gogui.EventCtx) {
		if look.Shown {
			p.Conceal()
		} else {
			p.Reveal()
		}
		e.Consume()
		e.Window.InvalidateLayout()
	}
	return gogui.Column(cfg)
}

func floatAt(x, y, w, h float32, v gogui.View) gogui.View {
	cfg := gogui.ContainerCfg{
		Float:        true,
		FloatOffsetX: x,
		FloatOffsetY: y,
		Width:        w,
		Height:       h,
		Sizing:       gogui.FixedFixed,
		Padding:      gogui.NoPadding,
		Content:      []gogui.View{v},
	}
	if h == 0 {
		cfg.Sizing = gogui.FixedFit
	}
	return gogui.Column(cfg)
}

func centredImage(src string, w, h float32) gogui.View {
	return gogui.Column(gogui.ContainerCfg{
		Sizing:  gogui.FillFill,
		Padding: gogui.NoPadding,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Content: []gogui.View{gogui.Image(gogui.ImageCfg{Src: src, Width: w, Height: h})},
	})
}

func backdrop(look videoplayer.Look, vw, vh int) gogui.View {
	side := float32(min(vw, vh)) * 2 / 5
	var content []gogui.View
	if look.Logo != nil {
		lw, lh := look.Logo.Size()
		iw, ih := side, side*float32(lh)/float32(max(lw, 1))
		if ih > side {
			iw, ih = side*float32(lw)/float32(max(lh, 1)), side
		}
		content = append(content, gogui.Image(gogui.ImageCfg{Src: still(look.Logo), Width: iw, Height: ih}))
	}
	if look.Note != "" {
		st := gogui.CurrentTheme().TextStyleDef
		st.Color = gogui.Color{R: 0xdd, G: 0xdd, B: 0xdd, A: 0xff}
		content = append(content, gogui.Text(gogui.TextCfg{
			Text:      look.Note,
			TextStyle: st,
			Mode:      gogui.TextModeWrap,
		}))
	}
	return gogui.Column(gogui.ContainerCfg{
		Sizing:  gogui.FillFill,
		Padding: gogui.PaddingLarge,
		Spacing: gogui.SpacingLarge,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Content: content,
	})
}

func spinner(vw, vh int, since time.Duration) gogui.View {
	r := float32(min(vw, vh)) / 14
	dot := max(r/5, 3)
	lead := int(since/spinStep) % spinDots
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		Width: float32(vw), Height: float32(vh),
		Version: uint64(lead),
		OnDraw: func(dc *gogui.DrawContext) {
			cx, cy := float32(vw)/2, float32(vh)/2
			for i := range spinDots {
				a := 2 * math.Pi * float64(i) / spinDots
				x := cx + r*float32(math.Sin(a))
				y := cy - r*float32(math.Cos(a))
				age := (lead - i + spinDots) % spinDots
				v := uint8(0xf0 - age*0xb0/spinDots)
				dc.FilledPolygon(disc(x, y, dot), gogui.Color{R: v, G: v, B: v, A: 0xff})
			}
		},
	})
}

func (a *App) videoHeader(p *videoplayer.Page) gogui.View {
	return gogui.Row(gogui.ContainerCfg{
		Sizing:  gogui.FillFit,
		Padding: gogui.PaddingLarge,
		Color:   videoScrim,
		Content: []gogui.View{button("video-back", gogui.IconArrowLeft, iconStyle(videoInk), gogui.Color{}, func(e gogui.EventCtx) {
			shell.Get().Remove(p)
		})},
	})
}

func (a *App) videoBand(w *gogui.Window, p *videoplayer.Page, look videoplayer.Look) gogui.View {
	c := look.Controls
	now := c.Now()
	vw, vh := w.WindowSize()

	title := now.Title
	if title == "" {
		title = c.Label()
	}
	head := gogui.CurrentTheme().TextStyleDef
	head.Color = videoInk
	quiet := gogui.CurrentTheme().TextStyleDef
	quiet.Color = videoQuiet

	words := []gogui.View{gogui.Label(title, head)}
	if sub := strings.Join(nonEmpty(now.Artist, now.Album), " · "); sub != "" {
		words = append(words, gogui.Label(sub, quiet))
	}
	heading := []gogui.View{gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Content: words})}
	if now.Mark != nil && vw > vh {
		mw, mh := now.Mark.Size()
		side := reach()
		heading = append([]gogui.View{gogui.Image(gogui.ImageCfg{Src: still(now.Mark), Width: side * float32(mw) / float32(max(mh, 1)), Height: side})}, heading...)
	}

	rows := []gogui.View{gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Spacing: gogui.SpacingMedium, VAlign: gogui.VAlignMiddle, Content: heading})}
	if now.Length > 0 {
		var sk media.Seeker = c
		var marks []videoplayer.Mark
		if mk, ok := c.(videoplayer.Marked); ok {
			marks = mk.Marks()
		}
		rows = append(rows, progress(w, now, sk, quiet, marks))
		if now.LiveWithin > 0 {
			rows = append(rows, live(now, c, quiet))
		}
	}
	rows = append(rows, a.videoTransport(p, now, c), loudness())

	return gogui.Column(gogui.ContainerCfg{
		ID:      "video-band",
		Sizing:  gogui.FillFit,
		Padding: gogui.PaddingLarge,
		Spacing: gogui.SpacingMedium,
		Color:   videoScrim,
		Content: rows,
		OnClick: func(e gogui.EventCtx) {
			p.Linger()
			e.Consume()
		},
	})
}

func live(now media.Now, c videoplayer.Controls, st gogui.TextStyle) gogui.View {
	fill := color(palette().Danger)
	var content []gogui.View
	if behind := now.Length - now.Elapsed; behind > now.LiveWithin {
		fill = gogui.Color{R: 0x60, G: 0x60, B: 0x60, A: 0xff}
		content = append(content, gogui.Label("-"+clockText(behind), st))
	}
	content = append(content, gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding}))
	pill := gogui.CurrentTheme().TextStyleDef
	pill.Color = videoInk
	length := now.Length
	content = append(content, pressable(gogui.Row, gogui.ContainerCfg{
		ID:      "video-live",
		Color:   fill,
		Radius:  gogui.RadiusLarge,
		Padding: gogui.PaddingSmall,
		Content: []gogui.View{gogui.Label(say.T("player.live"), pill)},
	}, func(gogui.EventCtx) {
		if c.CanSeek() {
			c.Seek(length)
		}
	}))
	return gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, VAlign: gogui.VAlignMiddle, Content: content})
}

func (a *App) videoTransport(p *videoplayer.Page, now media.Now, c videoplayer.Controls) gogui.View {
	type control struct {
		glyph    string
		do       func()
		primary  bool
		disabled bool
	}
	atLive := now.LiveWithin > 0 && now.Length-now.Elapsed <= now.LiveWithin
	middle := control{glyph: gogui.IconPlay, do: c.Play, primary: true}
	if now.Playing && !now.Paused {
		middle = control{glyph: gogui.IconPause, do: c.Pause, primary: true}
	}
	var shown []control
	if now.Can.Has(media.CanPrevious) {
		shown = append(shown, control{glyph: gogui.IconStepBackward, do: c.Previous})
	}
	if c.CanSeek() {
		shown = append(shown, control{glyph: gogui.IconBackward, do: func() { c.Seek(c.Now().Elapsed - videoSkip) }})
	}
	shown = append(shown, middle)
	if c.CanSeek() {
		shown = append(shown, control{glyph: gogui.IconForward, do: func() { c.Seek(c.Now().Elapsed + videoSkip) }, disabled: atLive})
	}
	if now.Can.Has(media.CanNext) {
		shown = append(shown, control{glyph: gogui.IconStepForward, do: c.Next})
	}
	if now.Can.Has(media.CanStop) {
		shown = append(shown, control{glyph: gogui.IconStop, do: func() {
			c.Stop()
			shell.Get().Remove(p)
		}})
	}

	t := gogui.CurrentTheme()
	var buttons []gogui.View
	for i, b := range shown {
		st, fill := t.TextStyleIconLarge, gogui.Color{R: 0x2a, G: 0x2a, B: 0x2a, A: 0xff}
		st.Color, st.Size = videoInk, reach()*0.45
		if b.primary {
			fill = t.Cfg.ColorAccent
			st.Color, st.Size = t.Cfg.ColorBackground, reach()*0.75
		}
		var do func(gogui.EventCtx)
		if b.disabled {
			fill = gogui.Color{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff}
			st.Color = gogui.Color{R: 0x55, G: 0x55, B: 0x55, A: 0xff}
		} else {
			act := b.do
			do = func(gogui.EventCtx) {
				p.Linger()
				act()
			}
		}
		buttons = append(buttons, button(fmt.Sprintf("video-%d", i), b.glyph, st, fill, do))
	}
	return gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, HAlign: gogui.HAlignCenter, VAlign: gogui.VAlignMiddle, Spacing: gogui.SpacingLarge, Content: buttons})
}

func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
