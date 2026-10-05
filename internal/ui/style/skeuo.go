package style

import (
	gogui "github.com/go-gui-org/go-gui/gui"
)

const Skeuo = "skeuo"

var skeuoStyle = Style{Name: Skeuo, Palette: "Linen", Family: sans, Kit: skeuo{}, Shape: func(c *gogui.ThemeCfg) {
	c.Radius, c.RadiusSmall, c.RadiusMedium, c.RadiusLarge = 8, 6, 8, 12
}}

type skeuo struct{}

var (
	chromeLight = gogui.RGB(0xf4, 0xf4, 0xf4)
	chromeDark  = gogui.RGB(0x9c, 0x9c, 0xa0)
)

func chrome(dc *gogui.DrawContext, cx, cy, r float32) {
	dc.FilledCircle(cx, cy+r*0.08, r, gogui.Black.WithOpacity(0.35))
	dc.FilledCircleGradient(cx, cy, r, dial(cx, cy-r, cx, cy+r, chromeLight, chromeDark))
	dc.Circle(cx, cy, r, gogui.Black.WithOpacity(0.35), 1)
}

func (skeuo) Toggle(id string, on bool) gogui.View {
	t := gogui.CurrentTheme().Cfg
	w, h := Reach()*1.5, Reach()*0.72
	var v uint64 = 1
	if on {
		v = 2
	}
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		ID: id, Width: w, Height: h, Sizing: gogui.FixedFixed, Version: v,
		OnDraw: func(dc *gogui.DrawContext) {
			base, cx := mix(t.ColorInterior, gogui.Black, 0.25), h/2
			if on {
				base, cx = t.ColorAccent, w-h/2
			}
			dc.FilledRoundedRectGradient(0, 0, w, h, h/2, dial(0, 0, 0, h, mix(base, gogui.Black, 0.35), mix(base, gogui.White, 0.2)))
			dc.RoundedRect(0.5, 0.5, w-1, h-1, h/2, gogui.Black.WithOpacity(0.45), 1)
			chrome(dc, cx, h/2, h*0.44)
		},
	})
}

func (skeuo) Slider(cfg *gogui.SliderCfg) { cfg.Look = groove(cfg.Vertical, nil) }

func (skeuo) Seek(marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	return groove(false, marks)
}

func (skeuo) Panel(cfg *gogui.ContainerCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius = gogui.RadiusLarge
	cfg.Gradient = vertical(mix(t.ColorPanel, gogui.White, 0.12), mix(t.ColorPanel, gogui.Black, 0.18))
	cfg.ColorBorder, cfg.SizeBorder = gogui.Black.WithOpacity(0.4), gogui.BorderPx(1)
	cfg.Shadow = &gogui.BoxShadow{Color: gogui.Black.WithOpacity(0.45), OffsetY: 4, BlurRadius: 12}
}

func (skeuo) Key(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	base := t.ColorPanel
	st.Color = t.TextStyleDef.Color
	if state == Chosen {
		base, st.Color = t.ColorAccent, t.ColorBackground
	}
	cfg.Radius = gogui.RadiusMedium
	cfg.Gradient = vertical(mix(base, gogui.White, 0.3), mix(base, gogui.Black, 0.15))
	cfg.ColorBorder, cfg.SizeBorder = gogui.Black.WithOpacity(0.4), gogui.BorderPx(1)
	cfg.Shadow = &gogui.BoxShadow{Color: gogui.Black.WithOpacity(0.35), OffsetY: 2, BlurRadius: 4}
	if state == Partial {
		cfg.Color = mix(cfg.Color, t.ColorBackground, 0.4)
	}
}

func groove(vertical bool, marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	t := gogui.CurrentTheme().Cfg
	thumb := t.SizeSliderThumb
	return func(s gogui.SliderLookState) gogui.SliderParts {
		track := gogui.DrawCanvasCfg{
			Sizing: gogui.FillFixed, Height: thumb, Version: version(s.Pct, len(marks)),
			OnDraw: func(dc *gogui.DrawContext) {
				long, across := dc.Width, dc.Height
				if vertical {
					long, across = dc.Height, dc.Width
				}
				half, run, bar := thumb/2, max(long-thumb, 1), across*0.3
				head := s.Pct
				if vertical {
					head = 1 - s.Pct
				}
				box := func(a, b float32, top, bottom gogui.Color) {
					if b <= a {
						return
					}
					if vertical {
						y0, y1 := long-(half+run*b), long-(half+run*a)
						x0 := (across - bar) / 2
						dc.FilledRoundedRectGradient(x0, y0, bar, y1-y0, bar/2, dial(x0, 0, x0+bar, 0, top, bottom))
						return
					}
					y0 := (across - bar) / 2
					dc.FilledRoundedRectGradient(half+run*a, y0, run*(b-a), bar, bar/2, dial(0, y0, 0, y0+bar, top, bottom))
				}
				box(0, 1, gogui.Black.WithOpacity(0.55), gogui.Black.WithOpacity(0.2))
				for _, m := range marks {
					box(m.From, max(m.To, m.From+0.004), mix(m.Color, gogui.White, 0.3), mix(m.Color, gogui.Black, 0.2))
				}
				box(0, head, mix(t.ColorAccent, gogui.White, 0.35), mix(t.ColorAccent, gogui.Black, 0.15))
			},
		}
		if vertical {
			track.Sizing, track.Width, track.Height = gogui.FixedFill, thumb, 0
		}
		return gogui.SliderParts{
			Track: gogui.DrawCanvas(track),
			Fill:  placeholder(1, 1),
			Handle: gogui.DrawCanvas(gogui.DrawCanvasCfg{
				Width: thumb, Height: thumb, Sizing: gogui.FixedFixed,
				OnDraw: func(dc *gogui.DrawContext) { chrome(dc, thumb/2, thumb/2, thumb*0.42) },
			}),
		}
	}
}

func (skeuo) Progress(cfg *gogui.ProgressBarCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.Color, cfg.ColorBar = gogui.RadiusLarge, gogui.Black.WithOpacity(0.4), t.ColorAccent
}

func (skeuo) Field(cfg *gogui.InputCfg) {
	cfg.Radius, cfg.SizeBorder = gogui.RadiusMedium, gogui.BorderPx(1)
}

func (k skeuo) Tab(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) { k.Key(cfg, st, state) }

func (skeuo) Tile(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	base := t.ColorPanel
	if cfg.Color.IsSet() {
		base = cfg.Color
	}
	cfg.Gradient = vertical(mix(base, gogui.White, 0.15), mix(base, gogui.Black, 0.15))
	cfg.ColorBorder, cfg.SizeBorder = gogui.Black.WithOpacity(0.35), gogui.BorderPx(1)
	cfg.Shadow = &gogui.BoxShadow{Color: gogui.Black.WithOpacity(0.35), OffsetY: 3, BlurRadius: 8}
	if state == Chosen {
		cfg.ColorBorder, cfg.SizeBorder = t.ColorAccent, gogui.BorderPx(2)
	}
}

func (skeuo) Row(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	switch state {
	case Chosen:
		cfg.Gradient = vertical(mix(t.ColorAccent, gogui.White, 0.55), mix(t.ColorAccent, gogui.White, 0.25))
		cfg.ColorBorder, cfg.SizeBorder = gogui.Black.WithOpacity(0.35), gogui.BorderPx(1)
	case Partial:
		cfg.ColorBorder, cfg.SizeBorder = t.ColorAccent.WithOpacity(0.6), gogui.BorderPx(1)
	}
}
