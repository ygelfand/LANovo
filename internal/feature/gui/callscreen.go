package gui

import (
	"fmt"
	"strings"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"
	sharedhomeview "github.com/ygelfand/libcountertop/pkg/display/homeview"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	sharedcall "github.com/ygelfand/libcountertop/pkg/media/call"
	"github.com/ygelfand/libcountertop/pkg/say"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/widget"
)

func callScreen(v *sharedcall.View) *Screen {
	return &Screen{Fixed: true, Clear: true, View: v, Build: callBody}
}

func callBody(w *gogui.Window) gogui.View {
	c, ok := call.Get().Now()
	if !ok {
		return gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFill})
	}
	t := gogui.CurrentTheme().Cfg
	vw, vh := w.WindowSize()

	name := t.TextStyleDef
	name.Size *= 2
	status := secondary()
	status.Size *= 1.3

	controls := []gogui.View{callButtons(vw, c)}
	if c.State == sharedcall.Talking {
		level := volume.Get().Level(config.StreamVoice)
		controls = append([]gogui.View{presentation.Row("call-volume", "call",
			widget.Row{Label: say.T("call.volume"), Kind: widget.Slider, Level: level},
			func(l int) { volume.Get().Set(config.StreamVoice, l) }, false)}, controls...)
	}
	bottomFor := func(width float32) gogui.View {
		return gogui.Column(gogui.ContainerCfg{
			Width:   min(width-2*reach(), reach()*12),
			Sizing:  gogui.FixedFit,
			Padding: gogui.NoPadding,
			Spacing: gogui.SpacingMedium,
			Content: controls,
		})
	}
	bottom := bottomFor(float32(vw))
	pad := gogui.NewPadding(reach(), reach(), reach(), reach())
	ink := t.TextStyleDef.Color
	heading := []gogui.View{gogui.Label(c.Peer.Name, name)}
	if c.State == sharedcall.Talking && c.RemoteMuted && !c.Video {
		heading = append(
			heading,
			mark("call-remote-muted", "micoff", ink, t.ColorBackground, name.Size),
		)
	}
	title := gogui.Row(
		gogui.ContainerCfg{
			Sizing:  gogui.FitFit,
			Padding: gogui.NoPadding,
			Spacing: gogui.SpacingMedium,
			VAlign:  gogui.VAlignMiddle,
			Content: heading,
		},
	)

	if c.Video && c.State == sharedcall.Talking {
		l := call.Get().Arrange(vw, vh)
		rect := func(r display.Rect) ui.Rect { return ui.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H} }
		var layers []gogui.View
		if c.RemoteCameraOff {
			layers = append(layers, placed(rect(l.Remote), gogui.Column(gogui.ContainerCfg{
				Sizing:  gogui.FillFill,
				Padding: gogui.NoPadding,
				HAlign:  gogui.HAlignCenter,
				VAlign:  gogui.VAlignMiddle,
				Color:   t.ColorBackground,
				Content: []gogui.View{
					mark(
						"call-remote-camera",
						"cameraoff",
						t.ColorTextSecondary,
						t.ColorBackground,
						float32(min(l.Remote.W, l.Remote.H))*0.25,
					),
				},
			})))
		}
		if c.RemoteMuted {
			side := reach() * 1.1
			inset := int(reach() * 0.4)
			layers = append(
				layers,
				placed(
					ui.Rect{
						X: l.Picture.X + inset,
						Y: l.Picture.Y + inset,
						W: int(side),
						H: int(side),
					},
					gogui.Column(gogui.ContainerCfg{
						Sizing:  gogui.FillFill,
						Padding: gogui.NoPadding,
						Radius:  gogui.RadiusMedium,
						HAlign:  gogui.HAlignCenter,
						VAlign:  gogui.VAlignMiddle,
						Color:   t.ColorBackground.WithOpacity(0.6),
						Content: []gogui.View{
							mark("call-remote-muted", "micoff", ink, t.ColorBackground, side*0.65),
						},
					}),
				),
			)
		}
		if c.CameraOff || c.Covered {
			layers = append(layers, placed(rect(l.Self), gogui.Column(gogui.ContainerCfg{
				Sizing:  gogui.FillFill,
				Padding: gogui.NoPadding,
				Radius:  gogui.RadiusMedium,
				HAlign:  gogui.HAlignCenter,
				VAlign:  gogui.VAlignMiddle,
				Color:   t.ColorBackground,
				Content: []gogui.View{
					mark(
						"call-self-camera",
						"cameraoff",
						t.ColorTextSecondary,
						t.ColorBackground,
						float32(min(l.Self.W, l.Self.H))*0.4,
					),
				},
			})))
		}
		inset := int(reach() * 0.4)
		badge := gogui.Column(gogui.ContainerCfg{
			Sizing:  gogui.FitFit,
			Padding: gogui.NewPadding(reach()*0.2, reach()*0.4, reach()*0.2, reach()*0.4),
			Radius:  gogui.RadiusMedium,
			HAlign:  gogui.HAlignRight,
			Color:   t.ColorBackground.WithOpacity(0.6),
			Content: []gogui.View{
				gogui.Label(c.Peer.Name, name),
				gogui.Label(callStatus(c), status),
			},
		})
		layers = append(
			layers,
			placed(
				ui.Rect{
					X: l.Picture.X,
					Y: l.Picture.Y + inset,
					W: l.Picture.W - inset,
					H: l.Picture.H / 2,
				},
				gogui.Row(gogui.ContainerCfg{
					Sizing:  gogui.FillFill,
					Padding: gogui.NoPadding,
					HAlign:  gogui.HAlignRight,
					VAlign:  gogui.VAlignTop,
					Content: []gogui.View{badge},
				}),
			),
		)
		layers = append(layers, placed(rect(l.Panel), gogui.Column(gogui.ContainerCfg{
			Sizing:  gogui.FillFill,
			Padding: pad,
			Spacing: gogui.SpacingSmall,
			Content: []gogui.View{
				gogui.Column(gogui.ContainerCfg{Sizing: gogui.FillFill, Padding: gogui.NoPadding}),
				gogui.Row(
					gogui.ContainerCfg{
						Sizing:  gogui.FillFit,
						Padding: gogui.NoPadding,
						HAlign:  gogui.HAlignCenter,
						Content: []gogui.View{bottomFor(float32(l.Panel.W))},
					},
				),
			},
		})))
		return gogui.Column(
			gogui.ContainerCfg{Sizing: gogui.FillFill, Padding: gogui.NoPadding, Content: layers},
		)
	}
	tall := min(float32(vh)*0.3, float32(vw)*0.5*pictureAspect)
	key, svg := devicePicture(c.Peer.Board)
	return gogui.Column(gogui.ContainerCfg{
		Sizing:  gogui.FillFill,
		Padding: pad,
		Spacing: gogui.SpacingLarge,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Color:   t.ColorBackground,
		Content: []gogui.View{
			gogui.Svg(
				gogui.SvgCfg{
					ID:      "call-" + key,
					SvgData: svg,
					Width:   tall / pictureAspect,
					Height:  tall,
				},
			),
			title,
			gogui.Label(callStatus(c), status),
			bottom,
		},
	})
}

func callButtons(vw int, c sharedcall.Call) gogui.View {
	t := gogui.CurrentTheme().Cfg
	glyph := iconStyle(t.TextStyleDef.Color)
	glyph.Size = reach() * 0.6
	words := t.TextStyleDef
	height := reach() * 1.4
	calls := call.Get()
	var buttons []gogui.View
	switch c.State {
	case sharedcall.Ringing:
		buttons = []gogui.View{
			presentation.Toolkit.WideKey(
				"call-decline",
				gogui.IconClose,
				glyph,
				height,
				style.Partial,
				func(gogui.EventCtx) { calls.Hangup() },
			),
		}
		if c.Video {
			buttons = append(
				buttons,
				presentation.Toolkit.WideKey(
					"call-answer-audio",
					gogui.IconPhone,
					glyph,
					height,
					style.Partial,
					func(gogui.EventCtx) { calls.AnswerAudio() },
				),
				presentation.Toolkit.WideKey(
					"call-answer-video",
					gogui.IconVideo,
					glyph,
					height,
					style.Chosen,
					func(gogui.EventCtx) { calls.Answer() },
				),
			)
		} else {
			buttons = append(
				buttons,
				presentation.Toolkit.WideKey(
					"call-answer",
					gogui.IconPhone,
					glyph,
					height,
					style.Chosen,
					func(gogui.EventCtx) { calls.Answer() },
				),
			)
		}
	case sharedcall.Calling:
		buttons = []gogui.View{
			presentation.Toolkit.WideKey(
				"call-cancel",
				gogui.IconClose,
				glyph,
				height,
				style.Partial,
				func(gogui.EventCtx) { calls.Hangup() },
			),
		}
	default:
		mute, state := say.T("call.mute"), style.Partial
		if c.Muted {
			mute, state = say.T("call.unmute"), style.Chosen
		}
		buttons = []gogui.View{
			presentation.Toolkit.WideKey(
				"call-mute",
				mute,
				words,
				height,
				state,
				func(gogui.EventCtx) { calls.Mute(!c.Muted) },
			),
		}
		if c.Video {
			camera, state := say.T("call.camera.off"), style.Partial
			if c.CameraOff {
				camera, state = say.T("call.camera.on"), style.Chosen
			}
			buttons = append(
				buttons,
				presentation.Toolkit.WideKey(
					"call-camera",
					camera,
					words,
					height,
					state,
					func(gogui.EventCtx) { calls.Camera(c.CameraOff) },
				),
			)
		}
		buttons = append(
			buttons,
			presentation.Toolkit.WideKey(
				"call-hangup",
				gogui.IconClose,
				glyph,
				height,
				style.Partial,
				func(gogui.EventCtx) { calls.Hangup() },
			),
		)
	}
	return gogui.Row(
		gogui.ContainerCfg{
			Sizing:  gogui.FillFit,
			Padding: gogui.NoPadding,
			Spacing: gogui.SpacingLarge,
			Content: buttons,
		},
	)
}

func callStatus(c sharedcall.Call) string {
	switch c.State {
	case sharedcall.Calling:
		return say.T("call.calling")
	case sharedcall.Ringing:
		if c.Video {
			return say.T("call.incoming.video")
		}
		return say.T("call.incoming")
	}
	d := time.Since(c.Since).Truncate(time.Second)
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func profileScreen(p *sharedcall.Profile) *Screen {
	title := p.ID
	if peer, ok := discovery.Get().Find(p.ID); ok {
		title = peer.Name
	}
	return &Screen{
		Title: title,
		View:  p,
		Build: func(w *gogui.Window) gogui.View { return profileBody(w, p) },
	}
}

func profileBody(w *gogui.Window, prof *sharedcall.Profile) gogui.View {
	p, ok := discovery.Get().Find(prof.ID)
	if !ok {
		return sharedhomeview.Column([]gogui.View{gogui.Label(say.T("call.gone"), secondary())})
	}
	t := gogui.CurrentTheme().Cfg
	vw, vh := w.WindowSize()
	tall := min(float32(vh)*0.28, float32(vw)*0.6*pictureAspect)
	key, svg := devicePicture(p.Board)

	addrs := make([]string, 0, len(p.Addrs))
	for _, a := range p.Addrs {
		addrs = append(addrs, a.String())
	}
	capable := say.T("call.no")
	if p.Video() {
		capable = say.T("call.yes")
	}
	details := []widget.Row{
		{Label: say.T("call.model"), Kind: widget.Plain, Value: p.Model},
		{Label: say.T("call.address"), Kind: widget.Plain, Value: strings.Join(addrs, ", ")},
		{Label: say.T("call.version"), Kind: widget.Plain, Value: p.Version},
		{Label: say.T("call.video"), Kind: widget.Plain, Value: capable},
		{
			Label: say.T("call.seen"),
			Kind:  widget.Plain,
			Value: time.Since(p.Seen).Round(time.Second).String(),
		},
	}
	rows := []gogui.View{
		gogui.Row(
			gogui.ContainerCfg{
				Sizing:  gogui.FillFit,
				Padding: gogui.NoPadding,
				HAlign:  gogui.HAlignCenter,
				Content: []gogui.View{
					gogui.Svg(
						gogui.SvgCfg{
							ID:      "profile-" + key,
							SvgData: svg,
							Width:   tall / pictureAspect,
							Height:  tall,
						},
					),
				},
			},
		),
	}
	for i, r := range details {
		rows = append(
			rows,
			presentation.Row(fmt.Sprintf("profile-%d", i), "profile", r, nil, false),
		)
	}

	glyph := iconStyle(t.TextStyleDef.Color)
	glyph.Size = reach() * 0.7
	height := reach() * 1.8
	var video func(gogui.EventCtx)
	if p.Video() && board.Current().CameraWidth > 0 {
		video = func(gogui.EventCtx) { dial(p, true) }
	}
	rows = append(rows, gogui.Row(gogui.ContainerCfg{
		Sizing:  gogui.FillFit,
		Padding: gogui.NewPadding(reach()*0.5, 0, 0, 0),
		Spacing: gogui.SpacingLarge,
		Content: []gogui.View{
			presentation.Toolkit.WideKey(
				"profile-audio",
				gogui.IconPhone,
				glyph,
				height,
				style.Partial,
				func(gogui.EventCtx) { dial(p, false) },
			),
			presentation.Toolkit.WideKey(
				"profile-video",
				gogui.IconVideo,
				glyph,
				height,
				style.Partial,
				video,
			),
		},
	}))
	return sharedhomeview.Column(rows)
}
