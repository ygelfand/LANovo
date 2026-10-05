package style

import (
	gogui "github.com/go-gui-org/go-gui/gui"
)

const Glass = "glass"

var glassStyle = Style{Name: Glass, Palette: "Midnight", Family: sans, Kit: glass{}, Shape: func(c *gogui.ThemeCfg) {
	c.Radius, c.RadiusSmall, c.RadiusMedium, c.RadiusLarge = 14, 10, 14, 24
}}

type glass struct{}

func (glass) Toggle(id string, on bool) gogui.View {
	t := gogui.CurrentTheme().Cfg
	w, h := Reach()*1.25, Reach()*0.74
	var v uint64 = 1
	if on {
		v = 2
	}
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		ID: id, Width: w, Height: h, Sizing: gogui.FixedFixed, Version: v,
		OnDraw: func(dc *gogui.DrawContext) {
			track, cx := t.TextStyleDef.Color.WithOpacity(0.18), h/2
			if on {
				track, cx = t.ColorAccent, w-h/2
			}
			dc.FilledRoundedRect(0, 0, w, h, h/2, track)
			dc.FilledCircle(cx, h/2+h*0.04, h*0.42, gogui.Black.WithOpacity(0.25))
			dc.FilledCircle(cx, h/2, h*0.42, gogui.White)
		},
	})
}

func (glass) Slider(cfg *gogui.SliderCfg) { cfg.Look = frosted(cfg.Vertical, nil) }

func (glass) Seek(marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	return frosted(false, marks)
}

func (glass) Panel(cfg *gogui.ContainerCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius = t.ColorPanel.WithOpacity(0.72), gogui.RadiusLarge
	cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color.WithOpacity(0.14), gogui.BorderPx(1)
	cfg.Shadow = &gogui.BoxShadow{Color: gogui.Black.WithOpacity(0.28), OffsetY: 10, BlurRadius: 32}
}

func (glass) Key(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, st.Color = t.TextStyleDef.Color.WithOpacity(0.1), gogui.RadiusPx(cfg.Width/2), t.TextStyleDef.Color
	if state == Chosen {
		cfg.Color, st.Color = t.TextStyleDef.Color.WithOpacity(0.9), t.ColorBackground
	}
	if state == Partial {
		cfg.Color = mix(cfg.Color, t.ColorBackground, 0.4)
	}
}

func frosted(vertical bool, marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	t := gogui.CurrentTheme().Cfg
	thick := t.SizeSliderThumb * 0.6
	ink := t.TextStyleDef.Color
	return func(s gogui.SliderLookState) gogui.SliderParts {
		track := gogui.DrawCanvasCfg{
			Sizing: gogui.FillFixed, Height: thick, Version: version(s.Pct, len(marks)),
			OnDraw: func(dc *gogui.DrawContext) {
				w, h := dc.Width, dc.Height
				r := min(w, h) / 2
				dc.FilledRoundedRect(0, 0, w, h, r, ink.WithOpacity(0.14))
				if vertical {
					f := h * (1 - s.Pct)
					dc.FilledRoundedRect(0, h-f, w, f, r, ink.WithOpacity(0.85))
					return
				}
				for _, m := range marks {
					dc.FilledRect(w*m.From, 0, max(w*(m.To-m.From), 1), h, m.Color.WithOpacity(0.7))
				}
				dc.FilledRoundedRect(0, 0, max(w*s.Pct, h), h, r, ink.WithOpacity(0.85))
			},
		}
		if vertical {
			track.Sizing, track.Width, track.Height = gogui.FixedFill, thick, 0
		}
		return gogui.SliderParts{Track: gogui.DrawCanvas(track), Fill: placeholder(1, 1), Handle: placeholder(thick, thick)}
	}
}

func (glass) Progress(cfg *gogui.ProgressBarCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.Color, cfg.ColorBar = gogui.RadiusLarge, t.TextStyleDef.Color.WithOpacity(0.14), t.TextStyleDef.Color.WithOpacity(0.85)
}

func (glass) Field(cfg *gogui.InputCfg) {
	cfg.Radius, cfg.SizeBorder = gogui.RadiusMedium, gogui.BorderPx(1)
}

func (glass) Tab(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, st.Color = t.TextStyleDef.Color.WithOpacity(0.08), gogui.RadiusPx(max(cfg.Height, Reach())/2), t.TextStyleDef.Color
	if state == Chosen {
		cfg.Color, st.Color = t.TextStyleDef.Color.WithOpacity(0.85), t.ColorBackground
	}
}

func (glass) Tile(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius = gogui.RadiusLarge
	cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color.WithOpacity(0.14), gogui.BorderPx(1)
	cfg.Shadow = &gogui.BoxShadow{Color: gogui.Black.WithOpacity(0.2), OffsetY: 6, BlurRadius: 18}
	if state == Chosen {
		cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color.WithOpacity(0.7), gogui.BorderPx(2)
	}
}

func (glass) Row(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius = gogui.RadiusMedium
	switch state {
	case Chosen:
		cfg.Color, cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color.WithOpacity(0.1), t.TextStyleDef.Color.WithOpacity(0.25), gogui.BorderPx(1)
	case Partial:
		cfg.ColorBorder, cfg.SizeBorder = t.TextStyleDef.Color.WithOpacity(0.18), gogui.BorderPx(1)
	}
}
