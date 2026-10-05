package style

import (
	"encoding/binary"
	"slices"

	gogui "github.com/go-gui-org/go-gui/gui"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	sans = "Go"
	mono = "Go Mono"
)

type Style struct {
	Name    string
	Family  string
	Palette string
	Shape   func(cfg *gogui.ThemeCfg)
	Kit     Kit
}

const ThemeDefault = "Default"

func Theme(style, theme string) string {
	if theme == ThemeDefault {
		return ByName(style).Palette
	}
	return theme
}

var All = []Style{standardStyle, materialStyle, glassStyle, einkStyle, metroStyle, terminalStyle, hifiStyle, neonStyle, skeuoStyle}

func init() {
	gogui.RegisterAppFontBytes(goregular.TTF)
	gogui.RegisterAppFontBytes(gomedium.TTF)
	gogui.RegisterAppFontBytes(weighted(gobold.TTF, 700))
	gogui.RegisterAppFontBytes(gomono.TTF)
	gogui.RegisterAppFontBytes(weighted(gomonobold.TTF, 700))
}

func ByName(name string) Style {
	for _, s := range All {
		if s.Name == name {
			return s
		}
	}
	return All[0]
}

func Names() []string {
	out := make([]string, 0, len(All))
	for _, s := range All {
		out = append(out, s.Name)
	}
	return out
}

func weighted(ttf []byte, weight uint16) []byte {
	out := slices.Clone(ttf)
	tables := int(binary.BigEndian.Uint16(out[4:]))
	for i := range tables {
		rec := out[12+16*i:]
		if string(rec[:4]) == "OS/2" {
			binary.BigEndian.PutUint16(out[binary.BigEndian.Uint32(rec[8:])+4:], weight)
		}
	}
	return out
}

type State int

const (
	Rest State = iota
	Partial
	Chosen
)

type Kit interface {
	Toggle(id string, on bool) gogui.View
	Slider(cfg *gogui.SliderCfg)
	Seek(marks []Span) func(gogui.SliderLookState) gogui.SliderParts
	Progress(cfg *gogui.ProgressBarCfg)
	Field(cfg *gogui.InputCfg)
	Panel(cfg *gogui.ContainerCfg)
	Key(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State)
	Tab(cfg *gogui.ContainerCfg, st *gogui.TextStyle, state State)
	Tile(cfg *gogui.ContainerCfg, state State)
	Row(cfg *gogui.ContainerCfg, state State)
}

type Span struct {
	From, To float32
	Color    gogui.Color
}

func Reach() float32 { return gogui.CurrentTheme().Cfg.TextStyleDef.Size * 2.4 }

func spanAt(ss []Span, at float32) (gogui.Color, bool) {
	for _, s := range ss {
		if at >= s.From && at < s.To {
			return s.Color, true
		}
	}
	return gogui.Color{}, false
}

func version(pct float32, marks int) uint64 { return uint64(pct*1e6) + uint64(marks)<<32 }

func placeholder(w, h float32) gogui.View {
	return gogui.Row(gogui.ContainerCfg{Width: w, Height: h, Sizing: gogui.FixedFixed, Padding: gogui.NoPadding})
}

func mix(a, b gogui.Color, f float32) gogui.Color {
	m := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*f) }
	return gogui.RGBA(m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), m(a.A, b.A))
}

func vertical(top, bottom gogui.Color) *gogui.GradientDef {
	return &gogui.GradientDef{Direction: gogui.GradientToBottom, Stops: []gogui.GradientStop{{Color: top, Pos: 0}, {Color: bottom, Pos: 1}}}
}

func dial(x0, y0, x1, y1 float32, top, bottom gogui.Color) *gogui.CanvasGradient {
	return &gogui.CanvasGradient{X1: x0, Y1: y0, X2: x1, Y2: y1, Stops: []gogui.GradientStop{{Color: top, Pos: 0}, {Color: bottom, Pos: 1}}}
}
