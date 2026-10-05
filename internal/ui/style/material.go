package style

import (
	"math"

	gogui "github.com/go-gui-org/go-gui/gui"
)

const Material = "material"

var materialStyle = Style{Name: Material, Palette: "Plum", Family: sans, Kit: material{}, Shape: func(c *gogui.ThemeCfg) {
	c.Radius, c.RadiusSmall, c.RadiusMedium, c.RadiusLarge = 16, 12, 16, 28
}}

type material struct{}

func (material) Toggle(id string, on bool) gogui.View {
	t := gogui.CurrentTheme().Cfg
	w, h := Reach()*1.35, Reach()*0.78
	var v uint64 = 1
	if on {
		v = 2
	}
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		ID: id, Width: w, Height: h, Sizing: gogui.FixedFixed, Version: v,
		OnDraw: func(dc *gogui.DrawContext) {
			if on {
				dc.FilledRoundedRect(0, 0, w, h, h/2, t.ColorAccent)
				dc.FilledCircle(w-h/2, h/2, h*0.36, t.ColorBackground)
				return
			}
			dc.FilledRoundedRect(0, 0, w, h, h/2, t.ColorInterior)
			dc.RoundedRect(1, 1, w-2, h-2, h/2-1, t.ColorTextSecondary, 2)
			dc.FilledCircle(h/2, h/2, h*0.24, t.ColorTextSecondary)
		},
	})
}

func (material) Slider(cfg *gogui.SliderCfg) { cfg.Look = tonal(cfg.Vertical, nil, false) }

func (material) Seek(marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	return tonal(false, marks, true)
}

func (material) Panel(cfg *gogui.ContainerCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius = t.ColorPanel, gogui.RadiusLarge
}

func (material) Key(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, st.Color = t.ColorAccent.WithOpacity(0.18), gogui.RadiusPx(cfg.Width/2), t.TextStyleDef.Color
	if state == Chosen {
		cfg.Color, cfg.Radius, st.Color = t.ColorAccent, gogui.RadiusPx(cfg.Width*0.3), t.ColorBackground
	}
	if state == Partial {
		cfg.Color = mix(cfg.Color, t.ColorBackground, 0.4)
	}
}

func tonal(vertical bool, marks []Span, wave bool) func(gogui.SliderLookState) gogui.SliderParts {
	t := gogui.CurrentTheme().Cfg
	thumb := t.SizeSliderThumb
	active, rest := t.ColorAccent, t.ColorAccent.WithOpacity(0.25)
	return func(s gogui.SliderLookState) gogui.SliderParts {
		track := gogui.DrawCanvasCfg{
			Sizing: gogui.FillFixed, Height: thumb, Version: version(s.Pct, len(marks)),
			OnDraw: func(dc *gogui.DrawContext) {
				long, across := dc.Width, dc.Height
				if vertical {
					long, across = dc.Height, dc.Width
				}
				half, run := thumb/2, max(long-thumb, 1)
				bar, gap := across*0.42, thumb*0.25
				head := s.Pct
				if vertical {
					head = 1 - s.Pct
				}
				at := half + run*head
				box := func(a, b float32, c gogui.Color) {
					if b <= a {
						return
					}
					if vertical {
						dc.FilledRoundedRect((across-bar)/2, long-b, bar, b-a, bar/2, c)
						return
					}
					dc.FilledRoundedRect(a, (across-bar)/2, b-a, bar, bar/2, c)
				}
				box(at+gap, long, rest)
				for _, m := range marks {
					a, b := max(half+run*m.From, at+gap), half+run*m.To
					box(a, b, m.Color)
				}
				if !wave || vertical {
					box(0, at-gap, active)
					return
				}
				amp, length := across*0.14, across*1.1
				var pts []float32
				for x := float32(0); x <= at-gap; x += 2 {
					pts = append(pts, x, across/2+amp*float32(math.Sin(float64(x/length*2*math.Pi))))
				}
				if len(pts) >= 4 {
					dc.Polyline(pts, active, bar*0.35)
				}
			},
		}
		if vertical {
			track.Sizing, track.Width, track.Height = gogui.FixedFill, thumb, 0
		}
		hw, hh := thumb*0.14, thumb
		if vertical {
			hw, hh = thumb, thumb*0.14
		}
		return gogui.SliderParts{
			Track: gogui.DrawCanvas(track),
			Fill:  placeholder(1, 1),
			Handle: gogui.DrawCanvas(gogui.DrawCanvasCfg{
				Width: hw, Height: hh, Sizing: gogui.FixedFixed,
				OnDraw: func(dc *gogui.DrawContext) { dc.FilledRoundedRect(0, 0, hw, hh, min(hw, hh)/2, active) },
			}),
		}
	}
}

func (material) Progress(cfg *gogui.ProgressBarCfg) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.Color, cfg.ColorBar = gogui.RadiusLarge, t.ColorAccent.WithOpacity(0.25), t.ColorAccent
}

func (material) Field(cfg *gogui.InputCfg) { cfg.Radius = gogui.RadiusLarge }

func (material) Tab(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Color, cfg.Radius, st.Color = t.ColorAccent.WithOpacity(0.12), gogui.RadiusPx(max(cfg.Height, Reach())/2), t.TextStyleDef.Color
	if state == Chosen {
		cfg.Color, st.Color = t.ColorAccent, t.ColorBackground
	}
}

func (material) Tile(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.SizeBorder = gogui.RadiusLarge, gogui.BorderPx(0)
	if state == Chosen {
		cfg.Color = t.ColorAccent.WithOpacity(0.2)
	}
}

func (material) Row(cfg *gogui.ContainerCfg, state State) {
	t := gogui.CurrentTheme().Cfg
	cfg.Radius, cfg.SizeBorder = gogui.RadiusLarge, gogui.BorderPx(0)
	switch state {
	case Chosen:
		cfg.Color = t.ColorAccent.WithOpacity(0.16)
	case Partial:
		cfg.Color = t.ColorAccent.WithOpacity(0.07)
	}
}
