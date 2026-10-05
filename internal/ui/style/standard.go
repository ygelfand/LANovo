package style

import (
	gogui "github.com/go-gui-org/go-gui/gui"
)

const Standard = "standard"

var standardStyle = Style{Name: Standard, Palette: "Paper", Family: sans, Shape: func(*gogui.ThemeCfg) {}, Kit: standard{}}

type standard struct{}

func (standard) Toggle(id string, on bool) gogui.View {
	return gogui.Switch(gogui.SwitchCfg{ID: id, Selected: on, FocusDisabled: true})
}

func (standard) Slider(*gogui.SliderCfg) {}

func (standard) Seek(marks []Span) func(gogui.SliderLookState) gogui.SliderParts {
	t := gogui.CurrentTheme().Cfg
	size, thumb := t.SizeSlider, t.SizeSliderThumb
	played := t.ColorSelect
	if !played.IsSet() {
		played = t.ColorAccent
	}
	return func(s gogui.SliderLookState) gogui.SliderParts {
		track := gogui.DrawCanvas(gogui.DrawCanvasCfg{
			Sizing:  gogui.FillFixed,
			Height:  size,
			Version: version(s.Pct, len(marks)),
			OnDraw: func(dc *gogui.DrawContext) {
				half, span := thumb/2, max(dc.Width-thumb, 1)
				at := func(f float32) float32 { return half + span*f }
				dc.FilledRoundedRect(0, 0, dc.Width, size, size/2, t.ColorInterior)
				head := half + span*s.Pct
				dc.FilledRoundedRect(0, 0, head, size, size/2, played)
				if head > size {
					dc.FilledRect(head-size/2, 0, size/2, size, played)
				}
				for _, m := range marks {
					x0 := at(m.From)
					x1 := max(at(m.To), x0+1)
					dc.FilledRect(x0, 0, x1-x0, size, m.Color)
				}
			},
		})
		return gogui.SliderParts{
			Track: track,
			Fill:  gogui.Row(gogui.ContainerCfg{Height: size, Sizing: gogui.FixedFixed, Padding: gogui.NoPadding}),
			Handle: gogui.DrawCanvas(gogui.DrawCanvasCfg{
				Sizing: gogui.FixedFixed,
				Width:  thumb,
				Height: thumb,
				OnDraw: func(dc *gogui.DrawContext) {
					w, edge := max(thumb/6, 4), max(thumb/24, 1.5)
					x := (thumb - w) / 2
					dc.FilledRoundedRect(x-edge, 0, w+2*edge, thumb, (w+2*edge)/2, t.ColorBackground)
					dc.FilledRoundedRect(x, edge, w, thumb-2*edge, w/2, t.TextStyleDef.Color)
				},
			}),
		}
	}
}

func (standard) Progress(*gogui.ProgressBarCfg) {}

func (standard) Field(*gogui.InputCfg) {}

func (standard) Panel(*gogui.ContainerCfg) {}

func (standard) Key(*gogui.ContainerCfg, *gogui.TextStyle, State) {}

func (standard) Tab(*gogui.ContainerCfg, *gogui.TextStyle, State) {}

func (standard) Tile(*gogui.ContainerCfg, State) {}

func (standard) Row(*gogui.ContainerCfg, State) {}
