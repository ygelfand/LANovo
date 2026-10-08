package gui

import (
	"github.com/go-gui-org/go-glyph"
	gogui "github.com/go-gui-org/go-gui/gui"
	"rsc.io/qr"

	"github.com/ygelfand/libcountertop/pkg/say"

	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

const (
	onboardCard   = 0.84
	onboardRadius = 0.045
	onboardPad    = 0.075
	onboardBand   = 0.014
	onboardTitle  = 0.105
	onboardAddr   = 0.058
	onboardHint   = 0.046
	onboardMark   = 0.080
)

func onboard(w *gogui.Window, addr string) gogui.View {
	brand := theme.Brand()
	vw, vh := w.WindowSize()
	cw, ch := float32(vw)*onboardCard, float32(vh)*onboardCard
	short := min(cw, ch)
	pad := short * onboardPad
	band := short * onboardBand
	inner := [2]float32{cw - 2*pad - band, ch - 2*pad}

	var body gogui.View
	if vw >= vh {
		half := inner[0] * 0.42
		gap := inner[0] / 16
		body = gogui.Row(gogui.ContainerCfg{
			Sizing:  gogui.FillFill,
			Padding: gogui.NoPadding,
			Spacing: gogui.SpacingPx(gap),
			VAlign:  gogui.VAlignMiddle,
			Content: []gogui.View{
				qrCode(web.AdoptURL, half, inner[1], brand),
				onboardWords(w, addr, inner[0]-half-gap, short, brand),
			},
		})
	} else {
		part := inner[1] * 0.48
		body = gogui.Column(gogui.ContainerCfg{
			Sizing:  gogui.FillFill,
			Padding: gogui.NoPadding,
			Content: []gogui.View{
				qrCode(web.AdoptURL, inner[0], part, brand),
				onboardWords(w, addr, inner[0], short, brand),
			},
		})
	}

	card := gogui.Row(gogui.ContainerCfg{
		Width:   cw,
		Height:  ch,
		Sizing:  gogui.FixedFixed,
		Color:   color(brand.Surface),
		Radius:  gogui.RadiusPx(short * onboardRadius),
		Padding: gogui.NoPadding,
		Clip:    true,
		Content: []gogui.View{
			gogui.Column(
				gogui.ContainerCfg{
					Width:   band,
					Sizing:  gogui.FixedFill,
					Padding: gogui.NoPadding,
					Color:   color(brand.Accent),
				},
			),
			gogui.Column(
				gogui.ContainerCfg{
					Sizing:  gogui.FillFill,
					Padding: gogui.NewPadding(pad, pad, pad, pad),
					Content: []gogui.View{body},
				},
			),
		},
	})
	return gogui.Column(gogui.ContainerCfg{
		Sizing:  gogui.FillFill,
		Padding: gogui.NoPadding,
		Color:   color(brand.Background),
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Content: []gogui.View{card},
	})
}

func qrCode(url string, w, h float32, brand theme.Theme) gogui.View {
	code, err := qr.Encode(url, qr.M)
	if err != nil {
		return gogui.Column(gogui.ContainerCfg{Width: w, Height: h, Sizing: gogui.FixedFixed})
	}
	side := min(w, h)
	scale := max(float32(int(side*0.88/float32(code.Size))), 1)
	quiet := scale * 3
	drawn := float32(code.Size)*scale + 2*quiet
	ink := color(brand.Text)
	return gogui.Column(gogui.ContainerCfg{
		Width:   w,
		Height:  h,
		Sizing:  gogui.FixedFixed,
		Padding: gogui.NoPadding,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Content: []gogui.View{gogui.DrawCanvas(gogui.DrawCanvasCfg{
			Width: drawn, Height: drawn,
			Version: uint64(len(url)),
			OnDraw: func(dc *gogui.DrawContext) {
				dc.FilledRoundedRect(0, 0, drawn, drawn, quiet, gogui.RGB(0xff, 0xff, 0xff))
				for y := range code.Size {
					for x := range code.Size {
						if code.Black(x, y) {
							dc.FilledRect(
								quiet+float32(x)*scale,
								quiet+float32(y)*scale,
								scale,
								scale,
								ink,
							)
						}
					}
				}
			},
		})},
	})
}

func onboardWords(w *gogui.Window, addr string, wide, short float32, brand theme.Theme) gogui.View {
	title := say.T("onboard.title")
	head := lettering(glyph.TypefaceBold, 0, brand.Text)
	head.Size = fitted(w, title, head, wide, short*onboardTitle)

	plate := brand.Accent.Blend(brand.Surface, 0.88)
	at := lettering(glyph.TypefaceBold, 0, brand.Accent)
	at.Size = fitted(w, addr, at, wide*0.9, short*onboardAddr)

	markH := short * onboardMark
	mark := ui.Logo()
	mw, mh := mark.Bounds().Dx(), mark.Bounds().Dy()
	return gogui.Column(gogui.ContainerCfg{
		Width:   wide,
		Sizing:  gogui.FixedFit,
		Padding: gogui.NoPadding,
		Spacing: gogui.SpacingPx(head.Size / 3),
		Content: []gogui.View{
			gogui.Image(
				gogui.ImageCfg{
					Src:    imageSrc("logo/light", mark),
					Width:  markH * float32(mw) / float32(max(mh, 1)),
					Height: markH,
				},
			),
			gogui.Text(gogui.TextCfg{Text: title, TextStyle: head, Mode: gogui.TextModeWrap}),
			gogui.Row(gogui.ContainerCfg{
				Color:   color(plate),
				Radius:  gogui.RadiusPx(at.Size),
				Padding: gogui.NewPadding(at.Size*0.3, at.Size/2, at.Size*0.3, at.Size/2),
				Content: []gogui.View{gogui.Label(addr, at)},
			}),
			gogui.Text(
				gogui.TextCfg{
					Text:      say.T("onboard.hint"),
					TextStyle: lettering(glyph.TypefaceRegular, short*onboardHint, brand.Muted),
					Mode:      gogui.TextModeWrap,
				},
			),
		},
	})
}
