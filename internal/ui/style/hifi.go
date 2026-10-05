package style

import (
	gogui "github.com/go-gui-org/go-gui/gui"
)

const Hifi = "hifi"

var hifiStyle = Style{Name: Hifi, Palette: "Braun", Family: sans, Kit: hifi{}, Shape: func(c *gogui.ThemeCfg) {
	c.Radius, c.RadiusSmall, c.RadiusMedium, c.RadiusLarge = 4, 3, 4, 6
}}

type hifi struct{}

func (hifi) Toggle(id string, on bool) gogui.View {
	t := gogui.CurrentTheme().Cfg
	w, h := Reach()*1.3, Reach()*0.62
	var v uint64 = 1
	if on {
		v = 2
	}
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		ID:      id,
		Width:   w,
		Height:  h,
		Sizing:  gogui.FixedFixed,
		Version: v,
		OnDraw: func(dc *gogui.DrawContext) {
			r, inset := h*0.18, h*0.08
			dc.FilledRoundedRect(0, 0, w, h, r, t.ColorTextSecondary.WithOpacity(0.35))
			kx, kw := inset, w/2-inset*1.5
			if on {
				kx = w/2 + inset/2
			}
			dc.FilledRoundedRect(kx, inset, kw, h-inset*2, r*0.7, t.ColorPanel)
			led := t.ColorTextSecondary.WithOpacity(0.5)
			if on {
				led = t.ColorAccent
			}
			dc.FilledCircle(kx+kw/2, h/2, h*0.12, led)
		},
	})
}

func (hifi) Slider(cfg *gogui.SliderCfg) { cfg.Look = fader(cfg.Vertical, nil) }

func (hifi) Seek(marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	return fader(false, marks)
}

func (hifi) Panel(cfg *gogui.ContainerCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius = t.ColorPanel, gogui.RadiusSmall
	cfg.ColorBorder, cfg.SizeBorder = t.ColorTextSecondary.WithOpacity(0.5), gogui.BorderPx(1)
}

func (hifi) Key(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius = t.ColorPanel, gogui.RadiusSmall
	cfg.ColorBorder, cfg.SizeBorder = t.ColorTextSecondary.WithOpacity(0.6), gogui.BorderPx(1)
	st.Color = t.TextStyleDef.Color
	if state == Chosen {
		cfg.Color, cfg.ColorBorder, st.Color = t.ColorAccent, t.ColorAccent, t.ColorBackground
	}
	if state == Partial {
		cfg.Color = mix(cfg.Color, t.ColorBackground, 0.4)
	}
}

func fader(vertical bool, marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	t := gogui.CurrentTheme().Cfg
	thumb := t.SizeSliderThumb
	ink, tick, played := t.TextStyleDef.Color, t.ColorTextSecondary, t.ColorAccent
	return func(s gogui.SliderLookState) gogui.SliderParts {
		track := gogui.DrawCanvasCfg{
			Sizing:  gogui.FillFixed,
			Height:  thumb,
			Version: version(s.Pct, len(marks)),
			OnDraw: func(dc *gogui.DrawContext) {
				long, across := dc.Width, dc.Height
				if vertical {
					long, across = dc.Height, dc.Width
				}
				half, run := thumb/2, max(long-thumb, 1)
				line, mid := max(across*0.06, 2), across/2
				along := func(f float32) float32 { return half + run*f }
				seg := func(a, b, width float32, c gogui.Color) {
					if vertical {
						dc.FilledRect(mid-width/2, long-along(b), width, along(b)-along(a), c)
						return
					}
					dc.FilledRect(along(a), mid-width/2, along(b)-along(a), width, c)
				}
				for i := 0; i <= 10; i++ {
					size := across * 0.18
					if i%5 == 0 {
						size = across * 0.32
					}
					p, edge := along(float32(i)/10), mid-across*0.42
					if vertical {
						dc.Line(edge, long-p, edge+size, long-p, tick, 1)
					} else {
						dc.Line(p, edge, p, edge+size, tick, 1)
					}
				}
				seg(0, 1, line, tick.WithOpacity(0.5))
				head := s.Pct
				if vertical {
					head = 1 - s.Pct
				}
				seg(0, head, line*1.6, played)
				for _, m := range marks {
					seg(m.From, max(m.To, m.From+0.002), line*1.6, m.Color)
				}
			},
		}
		if vertical {
			track.Sizing, track.Width, track.Height = gogui.FixedFill, thumb, 0
		}
		capW, capH := thumb*0.55, thumb
		if vertical {
			capW, capH = thumb, thumb*0.55
		}
		return gogui.SliderParts{
			Track: gogui.DrawCanvas(track),
			Fill:  placeholder(1, 1),
			Handle: gogui.DrawCanvas(gogui.DrawCanvasCfg{
				Width:  capW,
				Height: capH,
				Sizing: gogui.FixedFixed,
				OnDraw: func(dc *gogui.DrawContext) {
					r := min(capW, capH) * 0.18
					dc.FilledRoundedRect(0, 0, capW, capH, r, t.ColorPanel)
					dc.RoundedRect(0, 0, capW, capH, r, ink, 1.5)
					if vertical {
						dc.Line(capW*0.15, capH/2, capW*0.85, capH/2, played, 2)
					} else {
						dc.Line(capW/2, capH*0.15, capW/2, capH*0.85, played, 2)
					}
				},
			}),
		}
	}
}

func (hifi) Progress(cfg *gogui.ProgressBarCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.Color, cfg.ColorBar = gogui.RadiusSmall, t.ColorTextSecondary.WithOpacity(0.3), t.ColorAccent
}

func (hifi) Field(cfg *gogui.InputCfg) {
	cfg.Radius, cfg.SizeBorder = gogui.RadiusSmall, gogui.BorderPx(1)
}

func (k hifi) Tab(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) { k.Key(cfg, st, state) }

func (hifi) Tile(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius = gogui.RadiusSmall
	cfg.ColorBorder, cfg.SizeBorder = t.ColorTextSecondary.WithOpacity(0.5), gogui.BorderPx(1)
	if state == Chosen {
		cfg.ColorBorder, cfg.SizeBorder = t.ColorAccent, gogui.BorderPx(2)
	}
}

func (hifi) Row(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius = gogui.RadiusSmall
	switch state {
	case Chosen:
		cfg.Color, cfg.ColorBorder, cfg.SizeBorder = t.ColorAccent.WithOpacity(0.12), t.ColorAccent, gogui.BorderPx(1)
	case Partial:
		cfg.ColorBorder, cfg.SizeBorder = t.ColorTextSecondary, gogui.BorderPx(1)
	}
}
