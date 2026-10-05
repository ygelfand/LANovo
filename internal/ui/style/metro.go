package style

import (
	gogui "github.com/go-gui-org/go-gui/gui"
)

const Metro = "metro"

var metroStyle = Style{Name: Metro, Palette: "Ocean", Family: sans, Kit: metro{}, Shape: func(c *gogui.ThemeCfg) {
	c.Radius, c.RadiusSmall, c.RadiusMedium, c.RadiusLarge = 0, 0, 0, 0
}}

type metro struct{}

func (metro) Toggle(id string, on bool) gogui.View {
	t := gogui.CurrentTheme().Cfg
	w, h := Reach()*1.4, Reach()*0.6
	var v uint64 = 1
	if on {
		v = 2
	}
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		ID: id, Width: w, Height: h, Sizing: gogui.FixedFixed, Version: v,
		OnDraw: func(dc *gogui.DrawContext) {
			ink, inset := t.TextStyleDef.Color, h*0.16
			dc.Rect(1, 1, w-2, h-2, ink, 2)
			knob := h * 0.45
			x := inset
			if on {
				dc.FilledRect(inset, inset, w-2*inset, h-2*inset, t.ColorAccent)
				x = w - inset - knob
			}
			dc.FilledRect(x, 0, knob, h, ink)
		},
	})
}

func (metro) Slider(cfg *gogui.SliderCfg) { cfg.Look = flat(cfg.Vertical, nil) }

func (metro) Seek(marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	return flat(false, marks)
}

func (metro) Panel(cfg *gogui.ContainerCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, cfg.Shadow = t.ColorPanel, gogui.NoRadius, nil
}

func (metro) Key(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, st.Color = t.ColorPanel, gogui.NoRadius, t.TextStyleDef.Color
	if state == Chosen {
		cfg.Color, st.Color = t.ColorAccent, t.ColorBackground
	}
	if state == Partial {
		cfg.Color = mix(cfg.Color, t.ColorBackground, 0.4)
	}
}

func flat(vertical bool, marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	t := gogui.CurrentTheme().Cfg
	thumb := t.SizeSliderThumb
	ink, active, rest := t.TextStyleDef.Color, t.ColorAccent, t.ColorTextSecondary.WithOpacity(0.4)
	return func(s gogui.SliderLookState) gogui.SliderParts {
		track := gogui.DrawCanvasCfg{
			Sizing: gogui.FillFixed, Height: thumb, Version: version(s.Pct, len(marks)),
			OnDraw: func(dc *gogui.DrawContext) {
				long, across := dc.Width, dc.Height
				if vertical {
					long, across = dc.Height, dc.Width
				}
				half, run, bar := thumb/2, max(long-thumb, 1), across*0.32
				box := func(a, b float32, c gogui.Color) {
					if vertical {
						dc.FilledRect((across-bar)/2, long-(half+run*b), bar, run*(b-a), c)
						return
					}
					dc.FilledRect(half+run*a, (across-bar)/2, run*(b-a), bar, c)
				}
				box(0, 1, rest)
				for _, m := range marks {
					box(m.From, max(m.To, m.From+0.002), m.Color)
				}
				head := s.Pct
				if vertical {
					head = 1 - s.Pct
				}
				box(0, head, active)
			},
		}
		if vertical {
			track.Sizing, track.Width, track.Height = gogui.FixedFill, thumb, 0
		}
		hw, hh := thumb*0.32, thumb
		if vertical {
			hw, hh = thumb, thumb*0.32
		}
		return gogui.SliderParts{
			Track: gogui.DrawCanvas(track),
			Fill:  placeholder(1, 1),
			Handle: gogui.DrawCanvas(gogui.DrawCanvasCfg{
				Width: hw, Height: hh, Sizing: gogui.FixedFixed,
				OnDraw: func(dc *gogui.DrawContext) { dc.FilledRect(0, 0, hw, hh, ink) },
			}),
		}
	}
}

func (metro) Progress(cfg *gogui.ProgressBarCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.Color, cfg.ColorBar = gogui.NoRadius, t.ColorTextSecondary.WithOpacity(0.4), t.ColorAccent
}

func (metro) Field(cfg *gogui.InputCfg) {
	cfg.Radius, cfg.SizeBorder = gogui.NoRadius, gogui.BorderPx(2)
}

func (metro) Tab(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, st.Color = gogui.Color{}, gogui.NoRadius, t.ColorTextSecondary
	if state == Chosen {
		st.Color = t.TextStyleDef.Color
	}
}

func (metro) Tile(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.Shadow, cfg.SizeBorder = gogui.NoRadius, nil, gogui.BorderPx(0)
	if state == Chosen {
		cfg.Color = t.ColorAccent
	}
}

func (metro) Row(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.SizeBorder = gogui.NoRadius, gogui.BorderPx(0)
	switch state {
	case Chosen:
		cfg.Color = t.ColorAccent.WithOpacity(0.3)
	case Partial:
		cfg.Color = t.ColorAccent.WithOpacity(0.12)
	}
}
