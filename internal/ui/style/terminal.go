package style

import (
	gogui "github.com/go-gui-org/go-gui/gui"
)

const Terminal = "terminal"

var terminalStyle = Style{Name: Terminal, Palette: "Phosphor", Family: mono, Kit: terminal{}, Shape: func(c *gogui.ThemeCfg) {
	c.Radius, c.RadiusSmall, c.RadiusMedium, c.RadiusLarge = 0, 0, 0, 0
}}

type terminal struct{}

func (terminal) Toggle(id string, on bool) gogui.View {
	t := gogui.CurrentTheme().Cfg
	st := t.TextStyleDef
	st.Color = t.ColorTextSecondary
	word := "[OFF]"
	if on {
		st.Color, word = t.ColorAccent, "[ ON]"
	}
	return gogui.Label(word, st)
}

func (terminal) Slider(cfg *gogui.SliderCfg) { cfg.Look = blocks(cfg.Vertical, nil) }

func (terminal) Seek(marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	return blocks(false, marks)
}

func (terminal) Panel(cfg *gogui.ContainerCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, cfg.Shadow = t.ColorPanel, gogui.NoRadius, nil
	cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color, gogui.BorderPx(2)
}

func (terminal) Key(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius = t.ColorBackground, gogui.NoRadius
	cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color, gogui.BorderPx(2)
	st.Color = t.TextStyleDef.Color
	if state == Chosen {
		cfg.Color, st.Color = t.TextStyleDef.Color, t.ColorBackground
	}
	if state == Partial {
		cfg.ColorBorder = t.ColorTextSecondary
	}
}

func blocks(vertical bool, marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	t := gogui.CurrentTheme().Cfg
	thick := t.SizeSliderThumb * 0.8
	lit, dim := t.TextStyleDef.Color, t.ColorTextSecondary.WithOpacity(0.35)
	return func(s gogui.SliderLookState) gogui.SliderParts {
		cfg := gogui.DrawCanvasCfg{
			Sizing:  gogui.FillFixed,
			Height:  thick,
			Version: version(s.Pct, len(marks)),
			OnDraw: func(dc *gogui.DrawContext) {
				long, across := dc.Width, dc.Height
				if vertical {
					long, across = dc.Height, dc.Width
				}
				n := max(int(long/(across*0.6)), 1)
				step := long / float32(n)
				gap := max(step*0.18, 1)
				filled := s.Pct
				if vertical {
					filled = 1 - s.Pct
				}
				for i := range n {
					at := (float32(i) + 0.5) / float32(n)
					mark, marked := spanAt(marks, at)
					c := dim
					switch {
					case at <= filled && marked:
						c = mark
					case at <= filled:
						c = lit
					case marked:
						c = mark.WithOpacity(0.55)
					}
					if vertical {
						dc.FilledRect(0, dc.Height-float32(i+1)*step+gap/2, dc.Width, step-gap, c)
					} else {
						dc.FilledRect(float32(i)*step+gap/2, 0, step-gap, dc.Height, c)
					}
				}
			},
		}
		if vertical {
			cfg.Sizing, cfg.Width, cfg.Height = gogui.FixedFill, thick, 0
		}
		return gogui.SliderParts{Track: gogui.DrawCanvas(cfg), Fill: placeholder(1, 1), Handle: placeholder(thick, thick)}
	}
}

func (terminal) Progress(cfg *gogui.ProgressBarCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.Color, cfg.ColorBar = gogui.NoRadius, t.ColorTextSecondary.WithOpacity(0.35), t.TextStyleDef.Color
}

func (terminal) Field(cfg *gogui.InputCfg) {
	cfg.Radius, cfg.SizeBorder = gogui.NoRadius, gogui.BorderPx(2)
}

func (k terminal) Tab(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	k.Key(cfg, st, state)
}

func (terminal) Tile(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.Shadow = gogui.NoRadius, nil
	cfg.ColorBorder, cfg.SizeBorder = t.ColorTextSecondary, gogui.BorderPx(2)
	if state == Chosen {
		cfg.ColorBorder = t.TextStyleDef.Color
	}
}

func (terminal) Row(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius = gogui.NoRadius
	switch state {
	case Chosen:
		cfg.Color, cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color.WithOpacity(0.12), t.TextStyleDef.Color, gogui.BorderPx(2)
	case Partial:
		cfg.ColorBorder, cfg.SizeBorder = t.ColorTextSecondary, gogui.BorderPx(2)
	}
}
