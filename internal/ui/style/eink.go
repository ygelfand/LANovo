package style

import (
	gogui "github.com/go-gui-org/go-gui/gui"
)

const Eink = "eink"

var einkStyle = Style{Name: Eink, Palette: "Paper", Family: sans, Kit: eink{}, Shape: func(c *gogui.ThemeCfg) {
	c.Radius, c.RadiusSmall, c.RadiusMedium, c.RadiusLarge = 0, 0, 0, 2
}}

type eink struct{}

func (eink) Toggle(id string, on bool) gogui.View {
	t := gogui.CurrentTheme().Cfg
	side := Reach() * 0.55
	var v uint64 = 1
	if on {
		v = 2
	}
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		ID: id, Width: side, Height: side, Sizing: gogui.FixedFixed, Version: v,
		OnDraw: func(dc *gogui.DrawContext) {
			ink := t.TextStyleDef.Color
			dc.Rect(1, 1, side-2, side-2, ink, 2)
			if on {
				inset := side * 0.22
				dc.FilledRect(inset, inset, side-2*inset, side-2*inset, ink)
			}
		},
	})
}

func (eink) Slider(cfg *gogui.SliderCfg) { cfg.Look = rule(cfg.Vertical, nil) }

func (eink) Seek(marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	return rule(false, marks)
}

func (eink) Panel(cfg *gogui.ContainerCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, cfg.Shadow = t.ColorBackground, gogui.NoRadius, nil
	cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color, gogui.BorderPx(1)
}

func (eink) Key(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, st.Color = t.ColorBackground, gogui.NoRadius, t.TextStyleDef.Color
	cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color, gogui.BorderPx(1)
	if state == Chosen {
		cfg.Color, st.Color = t.TextStyleDef.Color, t.ColorBackground
	}
	if state == Partial {
		cfg.Color = mix(cfg.Color, t.ColorBackground, 0.4)
	}
}

func rule(vertical bool, marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	t := gogui.CurrentTheme().Cfg
	thumb := t.SizeSliderThumb
	ink := t.TextStyleDef.Color
	return func(s gogui.SliderLookState) gogui.SliderParts {
		track := gogui.DrawCanvasCfg{
			Sizing: gogui.FillFixed, Height: thumb, Version: version(s.Pct, len(marks)),
			OnDraw: func(dc *gogui.DrawContext) {
				long, across := dc.Width, dc.Height
				if vertical {
					long, across = dc.Height, dc.Width
				}
				half, run, mid := thumb/2, max(long-thumb, 1), across/2
				line := func(a, b, width float32, c gogui.Color) {
					if vertical {
						dc.FilledRect(mid-width/2, long-(half+run*b), width, run*(b-a), c)
						return
					}
					dc.FilledRect(half+run*a, mid-width/2, run*(b-a), width, c)
				}
				line(0, 1, 1, ink.WithOpacity(0.45))
				for _, m := range marks {
					line(m.From, max(m.To, m.From+0.002), across*0.22, ink.WithOpacity(0.3))
				}
				head := s.Pct
				if vertical {
					head = 1 - s.Pct
				}
				line(0, head, 3, ink)
			},
		}
		if vertical {
			track.Sizing, track.Width, track.Height = gogui.FixedFill, thumb, 0
		}
		dot := thumb * 0.45
		return gogui.SliderParts{
			Track: gogui.DrawCanvas(track),
			Fill:  placeholder(1, 1),
			Handle: gogui.DrawCanvas(gogui.DrawCanvasCfg{
				Width: dot, Height: dot, Sizing: gogui.FixedFixed,
				OnDraw: func(dc *gogui.DrawContext) { dc.FilledCircle(dot/2, dot/2, dot/2, ink) },
			}),
		}
	}
}

func (eink) Progress(cfg *gogui.ProgressBarCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.Color, cfg.ColorBar = gogui.NoRadius, t.TextStyleDef.Color.WithOpacity(0.15), t.TextStyleDef.Color
}

func (eink) Field(cfg *gogui.InputCfg) {
	cfg.Radius, cfg.SizeBorder = gogui.NoRadius, gogui.BorderPx(1)
}

func (k eink) Tab(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) { k.Key(cfg, st, state) }

func (eink) Tile(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, cfg.Shadow = t.ColorBackground, gogui.NoRadius, nil
	cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color, gogui.BorderPx(1)
	if state == Chosen {
		cfg.SizeBorder = gogui.BorderPx(3)
	}
}

func (eink) Row(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius = gogui.Color{}, gogui.NoRadius
	switch state {
	case Chosen:
		cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color, gogui.BorderPx(2)
	case Partial:
		cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color.WithOpacity(0.5), gogui.BorderPx(1)
	}
}
