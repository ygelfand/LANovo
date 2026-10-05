package style

import (
	gogui "github.com/go-gui-org/go-gui/gui"
)

const Neon = "neon"

var neonStyle = Style{Name: Neon, Palette: "Synthwave", Family: sans, Kit: neon{}, Shape: func(c *gogui.ThemeCfg) {
	c.Radius, c.RadiusSmall, c.RadiusMedium, c.RadiusLarge = 12, 8, 12, 18
}}

type neon struct{}

func glow(c gogui.Color) *gogui.BoxShadow {
	return &gogui.BoxShadow{Color: c.WithOpacity(0.6), BlurRadius: 22, Spread: 1}
}

func (neon) Toggle(id string, on bool) gogui.View {
	t := gogui.CurrentTheme().Cfg
	w, h := Reach()*1.3, Reach()*0.66
	var v uint64 = 1
	if on {
		v = 2
	}
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		ID: id, Width: w, Height: h, Sizing: gogui.FixedFixed, Version: v,
		OnDraw: func(dc *gogui.DrawContext) {
			c, cx := t.ColorTextSecondary, h/2
			if on {
				c, cx = t.ColorAccent, w-h/2
			}
			dc.RoundedRect(3, 3, w-6, h-6, (h-6)/2, c.WithOpacity(0.25), 6)
			dc.RoundedRect(3, 3, w-6, h-6, (h-6)/2, c, 2)
			if on {
				dc.FilledCircle(cx, h/2, h*0.32, c.WithOpacity(0.3))
				dc.FilledCircle(cx, h/2, h*0.22, c)
				return
			}
			dc.Circle(cx, h/2, h*0.22, c, 2)
		},
	})
}

func (neon) Slider(cfg *gogui.SliderCfg) { cfg.Look = equalizer(cfg.Vertical) }

func (neon) Seek(marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	t := gogui.CurrentTheme().Cfg
	thumb := t.SizeSliderThumb
	lit, dim := t.ColorAccent, t.ColorTextSecondary.WithOpacity(0.3)
	return func(s gogui.SliderLookState) gogui.SliderParts {
		track := gogui.DrawCanvas(gogui.DrawCanvasCfg{
			Sizing: gogui.FillFixed, Height: thumb, Version: version(s.Pct, len(marks)),
			OnDraw: func(dc *gogui.DrawContext) {
				half, run, mid := thumb/2, max(dc.Width-thumb, 1), dc.Height/2
				x := func(f float32) float32 { return half + run*f }
				dc.Line(x(0), mid, x(1), mid, dim, 2)
				for _, m := range marks {
					dc.Line(x(m.From), mid, x(max(m.To, m.From+0.002)), mid, m.Color.WithOpacity(0.3), 10)
					dc.Line(x(m.From), mid, x(max(m.To, m.From+0.002)), mid, m.Color, 3)
				}
				head := x(s.Pct)
				dc.Line(x(0), mid, head, mid, lit.WithOpacity(0.2), 14)
				dc.Line(x(0), mid, head, mid, lit.WithOpacity(0.4), 7)
				dc.Line(x(0), mid, head, mid, lit, 3)
			},
		})
		dot := thumb * 0.7
		return gogui.SliderParts{
			Track: track,
			Fill:  placeholder(1, 1),
			Handle: gogui.DrawCanvas(gogui.DrawCanvasCfg{
				Width: dot, Height: dot, Sizing: gogui.FixedFixed,
				OnDraw: func(dc *gogui.DrawContext) {
					dc.FilledCircle(dot/2, dot/2, dot/2, lit.WithOpacity(0.3))
					dc.FilledCircle(dot/2, dot/2, dot*0.28, lit)
				},
			}),
		}
	}
}

func (neon) Panel(cfg *gogui.ContainerCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius = t.ColorBackground, gogui.RadiusLarge
	cfg.ColorBorder, cfg.SizeBorder, cfg.Shadow = t.ColorAccent, gogui.BorderPx(2), glow(t.ColorAccent)
}

func (neon) Key(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, st.Color = t.ColorBackground, gogui.RadiusMedium, t.ColorAccent
	cfg.ColorBorder, cfg.SizeBorder, cfg.Shadow = t.ColorAccent, gogui.BorderPx(2), glow(t.ColorAccent)
	if state == Chosen {
		cfg.Color, st.Color = t.ColorAccent, t.ColorBackground
	}
	if state == Partial {
		cfg.Color = mix(cfg.Color, t.ColorBackground, 0.4)
	}
}

func equalizer(vertical bool) func(gogui.SliderLookState) gogui.SliderParts {
	t := gogui.CurrentTheme().Cfg
	thick := t.SizeSliderThumb
	lit, dim := t.ColorAccent, t.ColorTextSecondary.WithOpacity(0.25)
	return func(s gogui.SliderLookState) gogui.SliderParts {
		cfg := gogui.DrawCanvasCfg{
			Sizing: gogui.FillFixed, Height: thick, Version: version(s.Pct, 0),
			OnDraw: func(dc *gogui.DrawContext) {
				long, across := dc.Width, dc.Height
				if vertical {
					long, across = dc.Height, dc.Width
				}
				n := max(int(long/(across*0.32)), 2)
				step := long / float32(n)
				bar := step * 0.6
				filled := s.Pct
				if vertical {
					filled = 1 - s.Pct
				}
				for i := range n {
					f := (float32(i) + 0.5) / float32(n)
					size := across * (0.3 + 0.7*float32(i+1)/float32(n))
					c := dim
					if f <= filled {
						c = lit
					}
					if vertical {
						y := long - float32(i+1)*step + (step-bar)/2
						if f <= filled {
							dc.FilledRect((across-size)/2-3, y-3, size+6, bar+6, lit.WithOpacity(0.25))
						}
						dc.FilledRect((across-size)/2, y, size, bar, c)
						continue
					}
					x := float32(i)*step + (step-bar)/2
					if f <= filled {
						dc.FilledRect(x-3, across-size-3, bar+6, size+3, lit.WithOpacity(0.25))
					}
					dc.FilledRect(x, across-size, bar, size, c)
				}
			},
		}
		if vertical {
			cfg.Sizing, cfg.Width, cfg.Height = gogui.FixedFill, thick, 0
		}
		return gogui.SliderParts{Track: gogui.DrawCanvas(cfg), Fill: placeholder(1, 1), Handle: placeholder(thick*0.5, thick*0.5)}
	}
}

func (neon) Progress(cfg *gogui.ProgressBarCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.Color, cfg.ColorBar = gogui.RadiusLarge, t.ColorTextSecondary.WithOpacity(0.2), t.ColorAccent
}

func (neon) Field(cfg *gogui.InputCfg) {
	cfg.Radius, cfg.SizeBorder = gogui.RadiusMedium, gogui.BorderPx(2)
}

func (k neon) Tab(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) { k.Key(cfg, st, state) }

func (neon) Tile(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.ColorBorder, cfg.SizeBorder = t.ColorAccent.WithOpacity(0.45), gogui.BorderPx(1)
	if state == Chosen {
		cfg.ColorBorder, cfg.SizeBorder, cfg.Shadow = t.ColorAccent, gogui.BorderPx(2), glow(t.ColorAccent)
	}
}

func (neon) Row(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color = gogui.Color{}
	switch state {
	case Chosen:
		cfg.ColorBorder, cfg.SizeBorder, cfg.Shadow = t.ColorAccent, gogui.BorderPx(2), glow(t.ColorAccent)
	case Partial:
		cfg.ColorBorder, cfg.SizeBorder = t.ColorAccent.WithOpacity(0.5), gogui.BorderPx(1)
	}
}
