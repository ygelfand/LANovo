package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	"golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/assistant"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/timer"
	"github.com/ygelfand/LANovo/internal/feature/voice"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

const priorityAlert = 30

func messageCard(w *gogui.Window) gogui.View {
	m, ok := message.Get().Current()
	if !ok {
		return nil
	}
	t := gogui.CurrentTheme()
	vw, vh := w.WindowSize()
	tint := color(m.Tone.Color(palette()))

	words := []gogui.View{}
	if m.Title != "" {
		st := t.TextStyleDisplay
		st.Color = tint
		words = append(words, gogui.Text(gogui.TextCfg{Text: m.Title, TextStyle: st, Mode: gogui.TextModeWrap}))
	}
	words = append(words, gogui.Text(gogui.TextCfg{Text: m.Body, TextStyle: t.TextStyleTitle, Mode: gogui.TextModeWrap}))

	card := gogui.Row(panel(gogui.ContainerCfg{
		ID:        "message",
		Width:     float32(vw) * 0.82,
		MinHeight: float32(vh) * 0.4,
		Sizing:    gogui.FixedFit,
		VAlign:    gogui.VAlignMiddle,
		Padding:   gogui.PaddingLarge,
		Spacing:   gogui.SpacingLarge,
		Clip:      true,
		Content: []gogui.View{
			gogui.Column(gogui.ContainerCfg{Width: t.Cfg.RadiusMedium, Sizing: gogui.FixedFill, Color: tint, Radius: gogui.RadiusSmall, Padding: gogui.NoPadding}),
			gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, Spacing: gogui.SpacingMedium, Content: words}),
		},
	}))
	return gogui.Column(gogui.ContainerCfg{
		Width:   float32(vw),
		Height:  float32(vh),
		Sizing:  gogui.FixedFixed,
		Padding: gogui.NoPadding,
		Color:   gogui.Black.WithOpacity(0.5),
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Content: []gogui.View{card},
	})
}

const priorityNotice = 20

func timerCard(w *gogui.Window) gogui.View {
	c := timer.Get().Now()
	if !c.Showing {
		return nil
	}
	t := gogui.CurrentTheme()
	vw, vh := w.WindowSize()
	ink := t.Cfg.TextStyleDef.Color
	glyph := gogui.IconClock
	if c.Ringing {
		ink, glyph = t.Cfg.ColorError, gogui.IconBell
	}
	big := t.TextStyleTitle
	big.Color = ink

	line := []gogui.View{
		gogui.Label(glyph, iconStyle(ink)),
		gogui.Label(timer.Remaining(c.Left), big),
	}
	if c.Name != "" {
		line = append(line, gogui.Label(c.Name, secondary()))
	}
	content := []gogui.View{gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, VAlign: gogui.VAlignMiddle, Padding: gogui.NoPadding, Spacing: gogui.SpacingMedium, Content: line})}
	if !c.Ringing && c.Of > 0 {
		content = append(content, progressBar(gogui.ProgressBarCfg{Percent: float32(min(max(c.Left.Seconds()/c.Of.Seconds(), 0), 1)), Sizing: gogui.FillFit}))
	}

	var stop func(gogui.EventCtx)
	if c.Ringing {
		stop = func(gogui.EventCtx) { timer.Get().Stop() }
	}
	card := pressable(gogui.Column, panel(gogui.ContainerCfg{
		ID:      "timer",
		Width:   float32(vw) * 0.84,
		Sizing:  gogui.FixedFit,
		Padding: gogui.PaddingMedium,
		Spacing: gogui.SpacingSmall,
		Content: content,
	}), stop)
	return gogui.Column(gogui.ContainerCfg{
		Width:   float32(vw),
		Height:  float32(vh),
		Sizing:  gogui.FixedFixed,
		Padding: gogui.PaddingMedium,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignBottom,
		Content: []gogui.View{card},
	})
}

func assistantPanel(w *gogui.Window) gogui.View {
	show, ok := assistant.Get().Now()
	if !ok {
		return nil
	}
	t := gogui.CurrentTheme()
	vw, vh := w.WindowSize()
	look := assistant.LookFor(show)
	if !look.Custom() {
		assistant.Unstage()
	}
	if look.Custom() && look.Place == config.LookFull {
		assistant.Stage(ui.Rect{W: vw, H: vh}, assistant.BelowUI, look)
		return captions(w, show)
	}
	deep := float32(vh) * 0.26
	pad := t.Cfg.PaddingLarge.Left
	bandW, bandH := float32(vw)-2*pad, deep*0.34

	ink := color(palette().Text)
	switch show.Phase {
	case voice.Listening:
		ink = t.Cfg.ColorAccent
	case voice.Thinking:
		ink = t.Cfg.ColorTextSecondary
	}
	band := gogui.DrawCanvas(gogui.DrawCanvasCfg{
		ID: "assistant-band", Width: bandW, Height: bandH,
		Version: uint64(show.At.UnixNano()),
		OnDraw: func(dc *gogui.DrawContext) {
			if show.Phase == voice.Thinking {
				dots := assistant.Dots(show)
				size := min(bandH/3, bandW/float32(len(dots)*4))
				gap := size * 2
				left := (bandW - (size*float32(len(dots)) + gap*float32(len(dots)-1))) / 2
				for i, lift := range dots {
					dc.FilledCircle(left+float32(i)*(size+gap)+size/2, bandH/2-size*float32(lift), size/2, ink)
				}
				return
			}
			bars := assistant.Bars(show)
			n := float32(len(bars))
			gap := max(bandW/(n*4), 1)
			wide := max((bandW-gap*(n-1))/n, 1)
			left := (bandW - (wide*n + gap*(n-1))) / 2
			for i, part := range bars {
				high := max(bandH*float32(part), 1)
				dc.FilledRoundedRect(left+float32(i)*(wide+gap), (bandH-high)/2, wide, high, wide/2, ink)
			}
		},
	})

	if look.Custom() {
		share := float32(0.3)
		if vw > vh {
			share = 0.4
		}
		stageH := min(bandW/1.6, float32(vh)*share)
		stage := gogui.Row(gogui.ContainerCfg{
			ID: "assistant-stage", Width: stageH * 1.6, Height: stageH, Sizing: gogui.FixedFixed, Padding: gogui.NoPadding,
			AmendLayout: func(e gogui.EventCtx) {
				s := e.Layout.Shape
				assistant.Stage(ui.Rect{X: int(s.X), Y: int(s.Y), W: int(s.Width), H: int(s.Height)}, assistant.AboveUI, look)
			},
		})
		band = gogui.Row(gogui.ContainerCfg{Sizing: gogui.FillFit, Padding: gogui.NoPadding, HAlign: gogui.HAlignCenter, Content: []gogui.View{stage}})
	}
	content := []gogui.View{band}
	if show.Said != "" {
		content = append(content, gogui.Label(show.Said, secondary()))
	}
	if show.Reply != "" {
		content = append(content, gogui.Text(gogui.TextCfg{Text: assistant.Revealed(show.Reply, show.Reveal), TextStyle: t.TextStyleTitle, Mode: gogui.TextModeWrap}))
	}
	return gogui.Column(panel(gogui.ContainerCfg{
		ID:           "assistant",
		Float:        true,
		FloatOffsetY: deep * (float32(show.Down) - 1),
		Width:        float32(vw),
		MaxHeight:    float32(vh) * 0.7,
		Sizing:       gogui.FixedFit,
		Padding:      gogui.PaddingLarge,
		Spacing:      gogui.SpacingSmall,
		Clip:         true,
		Content:      content,
	}))
}

func captions(w *gogui.Window, show assistant.Showing) gogui.View {
	t := gogui.CurrentTheme()
	var lines []gogui.View
	if show.Said != "" {
		lines = append(lines, gogui.Text(gogui.TextCfg{Text: show.Said, TextStyle: secondary(), Mode: gogui.TextModeWrap}))
	}
	if show.Reply != "" {
		lines = append(lines, gogui.Text(gogui.TextCfg{Text: assistant.Revealed(show.Reply, show.Reveal), TextStyle: t.TextStyleTitle, Mode: gogui.TextModeWrap}))
	}
	var content []gogui.View
	if len(lines) > 0 {
		content = append(content, gogui.Column(gogui.ContainerCfg{
			ID:      "assistant-captions",
			Sizing:  gogui.FillFit,
			Color:   color(palette().Background).WithOpacity(0.6),
			Radius:  gogui.RadiusLarge,
			Padding: gogui.PaddingLarge,
			Spacing: gogui.SpacingSmall,
			Content: lines,
		}))
	}
	return gogui.Column(gogui.ContainerCfg{
		ID:      "assistant",
		Sizing:  gogui.FillFill,
		Padding: gogui.PaddingLarge,
		VAlign:  gogui.VAlignBottom,
		Content: content,
	})
}

func fullLook() bool {
	show, ok := assistant.Get().Now()
	if !ok {
		return false
	}
	l := assistant.LookFor(show)
	return l.Custom() && l.Place == config.LookFull && assistant.Staged()
}

const priorityMarks = 100

var markRed = theme.Color{R: 0xe0, G: 0x3b, B: 0x2f}

func privacyMarks(w *gogui.Window) gogui.View {
	m := privacy.Get().Current()
	if !m.Showing() {
		return nil
	}
	vw, vh := w.WindowSize()
	size := float32(min(vw, vh)) / 11
	glyph := int(size * 0.30)
	at := int(size/3) - glyph/2
	red := color(markRed)

	corner := func(right bool, icon ui.Icon, key string) []gogui.View {
		x := float32(0)
		gx := at
		if right {
			x = float32(vw) - size*1.3
			gx = vw - at - glyph
		}
		room := size * 1.3
		tri := gogui.DrawCanvas(gogui.DrawCanvasCfg{
			Width: room, Height: room,
			OnDraw: func(dc *gogui.DrawContext) {
				shape := func(reach, drop float32) []float32 {
					if right {
						return []float32{room, drop, room - reach, drop, room, reach + drop}
					}
					return []float32{0, drop, reach, drop, 0, reach + drop}
				}
				for k := 4; k >= 1; k-- {
					dc.FilledPolygon(shape(size+float32(k)*size*0.05, 2), gogui.Black.WithOpacity(0.12))
				}
				dc.FilledPolygon(shape(size, 0), red)
			},
		})
		mark := picture(painted("privacy/"+key, glyph, glyph, markRed, func(s ui.Surface, r ui.Rect, _ theme.Theme) {
			ui.DrawIcon(s, icon, r, theme.Color{R: 0xff, G: 0xff, B: 0xff}, markRed)
		}), glyph, glyph)
		return []gogui.View{
			placed(ui.Rect{X: int(x), Y: 0, W: int(size * 1.3), H: int(size * 1.3)}, tri),
			placed(ui.Rect{X: gx, Y: at, W: glyph, H: glyph}, mark),
		}
	}

	var layers []gogui.View
	if m.MicMuted {
		layers = append(layers, corner(false, icons.AVMicOff, "mic")...)
	}
	if m.CameraBlocked {
		layers = append(layers, corner(true, icons.AVVideocamOff, "camera")...)
	}
	return gogui.Column(gogui.ContainerCfg{
		Width:   float32(vw),
		Height:  float32(vh),
		Sizing:  gogui.FixedFixed,
		Padding: gogui.NoPadding,
		Content: layers,
	})
}
