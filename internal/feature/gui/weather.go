package gui

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/go-gui-org/go-glyph"
	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/weather"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/libcountertop/pkg/say"
)

type weatherView func(w *gogui.Window, size float32, r weather.Reading, ink theme.Theme) gogui.View

var weatherLooks = map[config.WeatherLook]weatherView{
	config.WeatherCompact: compactWeather,
	config.WeatherStack:   stackWeather,
	config.WeatherIcon:    iconWeather,
	config.WeatherWords:   wordsWeather,
	config.WeatherCard:    cardWeather,
	config.WeatherDetail:  detailWeather,
}

var (
	weatherStart = time.Now()
	ticking      atomic.Bool
)

func phase(w *gogui.Window) float64 {
	if !config.Get().Weather.Animate {
		return 0
	}
	if !ticking.Swap(true) {
		time.AfterFunc(weatherFrame, func() {
			ticking.Store(false)
			w.QueueCommand(func(w *gogui.Window) { w.InvalidateLayout() })
		})
	}
	return time.Since(weatherStart).Seconds()
}

func weatherLayer(w *gogui.Window, vw, vh int, top float32, ink theme.Theme) gogui.View {
	r, ok := weather.Get().Now()
	if !ok {
		return nil
	}
	r.Phase = phase(w)
	wc := config.Get().Weather
	build, ok := weatherLooks[wc.Look]
	if !ok {
		build = compactWeather
	}
	size := reach() * float32(0.5+wc.Size.Share()*1.4)
	margin := reach() * 0.5
	v := gogui.VAlignMiddle
	switch wc.Position {
	case config.PositionTop:
		v = gogui.VAlignTop
	case config.PositionBottom:
		v = gogui.VAlignBottom
	}
	return placed(ui.Rect{W: vw, H: vh}, gogui.Column(gogui.ContainerCfg{
		Sizing:  gogui.FillFill,
		Padding: gogui.NewPadding(top+margin, margin, margin, margin),
		HAlign:  alignOf(),
		VAlign:  v,
		Content: []gogui.View{build(w, size, r, ink)},
	}))
}

func weatherSample(w *gogui.Window, look config.WeatherLook, tw, th float32) gogui.View {
	r, ok := weather.Get().Now()
	if !ok {
		t := 72.0
		r = weather.Reading{Condition: "sunny", Temperature: &t}
	}
	r.Phase = phase(w)
	build, ok := weatherLooks[look]
	if !ok {
		build = compactWeather
	}
	return gogui.Column(gogui.ContainerCfg{
		Width:   tw,
		Height:  th,
		Sizing:  gogui.FixedFixed,
		Padding: gogui.NoPadding,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Clip:    true,
		Content: []gogui.View{build(w, th*0.28, r, palette())},
	})
}

func degrees(r weather.Reading) string {
	if r.Temperature == nil {
		return "–"
	}
	return fmt.Sprintf("%.0f°", *r.Temperature)
}

func condition(r weather.Reading) string {
	if s := say.T("weather.cond." + r.Condition); s != "weather.cond."+r.Condition {
		return s
	}
	return r.Condition
}

func alignOf() gogui.HorizontalAlign {
	switch config.Get().Weather.Align {
	case config.AlignLeft:
		return gogui.HAlignLeft
	case config.AlignRight:
		return gogui.HAlignRight
	}
	return gogui.HAlignCenter
}

func stacked(content []gogui.View) gogui.View {
	return gogui.Column(gogui.ContainerCfg{Padding: gogui.NoPadding, HAlign: alignOf(), Content: content})
}

func line(gap float32, content ...gogui.View) gogui.View {
	return gogui.Row(gogui.ContainerCfg{Padding: gogui.NoPadding, VAlign: gogui.VAlignMiddle, Spacing: gogui.SpacingPx(gap), Content: content})
}

func compactWeather(_ *gogui.Window, size float32, r weather.Reading, ink theme.Theme) gogui.View {
	big := lettering(glyph.TypefaceBold, size, ink.Text)
	return line(size*0.2, weatherIcon(r.Condition, r.Phase, size*1.1, ink), gogui.Label(degrees(r), big))
}

func stackWeather(_ *gogui.Window, size float32, r weather.Reading, ink theme.Theme) gogui.View {
	big := lettering(glyph.TypefaceBold, size*1.3, ink.Text)
	small := lettering(glyph.TypefaceRegular, size*0.4, ink.Muted)
	return stacked([]gogui.View{
		gogui.Label(degrees(r), big),
		line(small.Size*0.4, weatherIcon(r.Condition, r.Phase, small.Size*1.4, ink), gogui.Label(condition(r), small)),
	})
}

func iconWeather(_ *gogui.Window, size float32, r weather.Reading, ink theme.Theme) gogui.View {
	st := lettering(glyph.TypefaceBold, size*0.7, ink.Text)
	return stacked([]gogui.View{weatherIcon(r.Condition, r.Phase, size*1.8, ink), gogui.Label(degrees(r), st)})
}

func wordsWeather(_ *gogui.Window, size float32, r weather.Reading, ink theme.Theme) gogui.View {
	return gogui.Label(condition(r)+", "+degrees(r), lettering(glyph.TypefaceRegular, size*0.6, ink.Text))
}

func cardWeather(_ *gogui.Window, size float32, r weather.Reading, ink theme.Theme) gogui.View {
	big := lettering(glyph.TypefaceBold, size, ink.Text)
	small := lettering(glyph.TypefaceRegular, size*0.35, ink.Muted)
	words := []gogui.View{gogui.Label(degrees(r), big), gogui.Label(condition(r), small)}
	if r.Humidity != nil {
		words = append(words, gogui.Label(say.F("weather.humidity", map[string]any{"N": fmt.Sprintf("%.0f", *r.Humidity)}), small))
	}
	return gogui.Row(panel(gogui.ContainerCfg{
		Padding: gogui.PaddingLarge,
		Spacing: gogui.SpacingPx(size * 0.3),
		VAlign:  gogui.VAlignMiddle,
		Content: []gogui.View{
			weatherIcon(r.Condition, r.Phase, size*1.5, ink),
			gogui.Column(gogui.ContainerCfg{Padding: gogui.NoPadding, Content: words}),
		},
	}))
}

func detailWeather(_ *gogui.Window, size float32, r weather.Reading, ink theme.Theme) gogui.View {
	big := lettering(glyph.TypefaceBold, size, ink.Text)
	small := lettering(glyph.TypefaceRegular, size*0.33, ink.Muted)
	lines := []gogui.View{line(size*0.2, weatherIcon(r.Condition, r.Phase, size, ink), gogui.Label(degrees(r), big)), gogui.Label(condition(r), small)}
	if r.Feels != nil {
		lines = append(lines, gogui.Label(say.F("weather.feels", map[string]any{"T": fmt.Sprintf("%.0f°", *r.Feels)}), small))
	}
	if r.Humidity != nil {
		lines = append(lines, gogui.Label(say.F("weather.humidity", map[string]any{"N": fmt.Sprintf("%.0f", *r.Humidity)}), small))
	}
	if r.Wind != nil {
		lines = append(lines, gogui.Label(say.F("weather.wind", map[string]any{"N": fmt.Sprintf("%.0f", *r.Wind), "Unit": r.WindUnit}), small))
	}
	if r.UV != nil {
		lines = append(lines, gogui.Label(say.F("weather.uv", map[string]any{"N": fmt.Sprintf("%.0f", *r.UV)}), small))
	}
	return stacked(lines)
}

const weatherFrame = time.Second / 12

type tints struct {
	sun, moon, cloud, drop, flake, bolt, fog, wind, alarm gogui.Color
}

var natural = tints{
	sun:   gogui.RGB(0xff, 0xc1, 0x2e),
	moon:  gogui.RGB(0xf2, 0xe6, 0xb8),
	cloud: gogui.RGB(0xa9, 0xb4, 0xbf),
	drop:  gogui.RGB(0x3d, 0x9b, 0xf2),
	flake: gogui.RGB(0x8e, 0xc8, 0xf0),
	bolt:  gogui.RGB(0xff, 0xd8, 0x3a),
	fog:   gogui.RGB(0x9a, 0xa4, 0xae),
	wind:  gogui.RGB(0x84, 0xae, 0xcc),
	alarm: gogui.RGB(0xff, 0x6a, 0x3d),
}

func iconColors(ink theme.Theme) tints {
	if !config.Get().Weather.Themed {
		return natural
	}
	accent, text, second := color(ink.Accent), color(ink.Text), color(ink.Accent2)
	return tints{sun: accent, moon: accent, cloud: text, drop: second, flake: text, bolt: accent, fog: text, wind: text, alarm: accent}
}

func weatherIcon(cond string, p float64, side float32, ink theme.Theme) gogui.View {
	k := iconColors(ink)
	sun, body, drop := k.sun, k.cloud, k.drop
	frac := func(rate float64) float32 { return float32(p*rate - math.Floor(p*rate)) }
	sway := func(rate float64) float32 { return float32(math.Sin(p * rate)) }
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		Width:   side,
		Height:  side,
		Sizing:  gogui.FixedFixed,
		Version: uint64(len(cond)) + uint64(side)<<8 + uint64(p*12)<<24,
		OnDraw: func(dc *gogui.DrawContext) {
			s := side
			cloud := func(cx, cy, w float32) { drawCloud(dc, cx+sway(0.8)*s*0.03, cy, w, body) }
			moon, flake, bolt, fog, wind, alarm := k.moon, k.flake, k.bolt, k.fog, k.wind, k.alarm
			switch cond {
			case "sunny":
				drawSun(dc, s/2, s/2, s*0.22, s*0.42, sun, s*0.06, p*0.4)
			case "clear-night":
				drawMoon(dc, s*0.46, s*0.54, s*0.32, moon)
				for i, st := range [][2]float32{{0.8, 0.22}, {0.86, 0.6}, {0.62, 0.12}} {
					glow := float32(1)
					if p != 0 {
						glow = 0.25 + 0.75*(0.5+0.5*sway(1.3+float64(i)*0.7))
					}
					drawStar(dc, s*st[0], s*st[1], s*0.07, moon.WithOpacity(glow))
				}
			case "partlycloudy":
				drawSun(dc, s*0.38, s*0.36, s*0.15, s*0.3, sun, s*0.05, p*0.4)
				cloud(s*0.58, s*0.62, s*0.5)
			case "cloudy":
				cloud(s/2, s*0.55, s*0.8)
			case "fog":
				cloud(s/2, s*0.4, s*0.7)
				for i := range 3 {
					y := s*0.66 + float32(i)*s*0.11
					dx := sway(0.9+float64(i)*0.3) * s * 0.06
					dc.Line(s*0.2+dx, y, s*0.8+dx, y, fog, s*0.05)
				}
			case "rainy", "pouring":
				cloud(s/2, s*0.4, s*0.75)
				n, rate := 3, 1.4
				if cond == "pouring" {
					n, rate = 5, 2.4
				}
				drawDrops(dc, s, n, drop, frac(rate))
			case "snowy":
				cloud(s/2, s*0.4, s*0.75)
				drawFlakes(dc, s, 3, flake, frac(0.5), sway(1.2))
			case "snowy-rainy":
				cloud(s/2, s*0.4, s*0.75)
				drawDrops(dc, s, 2, drop, frac(1.4))
				drawFlakes(dc, s, 2, flake, frac(0.5), sway(1.2))
			case "hail":
				cloud(s/2, s*0.4, s*0.75)
				for i := range 3 {
					y := s * (0.72 + 0.16*frac(1.6+float64(i)*0.13))
					dc.FilledCircle(s*(0.3+0.2*float32(i)), y, s*0.06, flake)
				}
			case "lightning", "lightning-rainy":
				cloud(s/2, s*0.4, s*0.75)
				if p != 0 && frac(0.45) > 0.12 {
					bolt = bolt.WithOpacity(0.35)
				}
				dc.FilledPolygon([]float32{s * 0.6, s * 0.52, s * 0.38, s * 0.78, s * 0.56, s * 0.74}, bolt)
				dc.FilledPolygon([]float32{s * 0.48, s * 0.7, s * 0.66, s * 0.68, s * 0.44, s * 0.98}, bolt)
				if cond == "lightning-rainy" {
					drawDrops(dc, s, 2, drop, frac(1.4))
				}
			case "windy", "windy-variant":
				for i, l := range []float32{0.7, 0.55, 0.8} {
					l *= 1 + 0.12*sway(1.1+float64(i)*0.4)
					y := s * (0.32 + 0.18*float32(i))
					dc.Line(s*0.12, y, s*(0.12+l*0.7), y, wind, s*0.06)
					dc.Arc(s*(0.12+l*0.7), y-s*0.07, s*0.07, s*0.07, math.Pi/2, -math.Pi*1.4, wind, s*0.06)
				}
			case "exceptional":
				dc.FilledPolygon([]float32{s / 2, s * 0.1, s * 0.92, s * 0.88, s * 0.08, s * 0.88}, alarm)
			default:
				cloud(s/2, s*0.55, s*0.8)
			}
		},
	})
}

func drawSun(dc *gogui.DrawContext, cx, cy, r, ray float32, c gogui.Color, width float32, turn float64) {
	dc.FilledCircle(cx, cy, r, c)
	for i := range 8 {
		a := float64(i)*math.Pi/4 + turn
		x0, y0 := cx+float32(math.Cos(a))*r*1.35, cy+float32(math.Sin(a))*r*1.35
		x1, y1 := cx+float32(math.Cos(a))*ray, cy+float32(math.Sin(a))*ray
		dc.Line(x0, y0, x1, y1, c, width)
	}
}

func drawMoon(dc *gogui.DrawContext, cx, cy, r float32, c gogui.Color) {
	const steps = 32
	at := func(i int, k float32) (float32, float32) {
		a := math.Pi/2 + math.Pi*float64(i)/steps
		return cx + float32(math.Cos(a))*r*k, cy - float32(math.Sin(a))*r
	}
	for i := range steps {
		ox0, oy0 := at(i, 1)
		ox1, oy1 := at(i+1, 1)
		ix0, iy0 := at(i, 0.45)
		ix1, iy1 := at(i+1, 0.45)
		dc.FilledPolygon([]float32{ox0, oy0, ox1, oy1, ix1, iy1, ix0, iy0}, c)
	}
}

func drawStar(dc *gogui.DrawContext, cx, cy, r float32, c gogui.Color) {
	w := r * 0.28
	dc.FilledPolygon([]float32{cx, cy - r, cx + w, cy, cx, cy + r, cx - w, cy}, c)
	dc.FilledPolygon([]float32{cx - r, cy, cx, cy - w, cx + r, cy, cx, cy + w}, c)
}

func drawCloud(dc *gogui.DrawContext, cx, cy, w float32, c gogui.Color) {
	h := w * 0.42
	dc.FilledRoundedRect(cx-w/2, cy-h*0.1, w, h*0.55, h*0.27, c)
	dc.FilledCircle(cx-w*0.18, cy, h*0.38, c)
	dc.FilledCircle(cx+w*0.08, cy-h*0.22, h*0.5, c)
	dc.FilledCircle(cx+w*0.3, cy+h*0.05, h*0.32, c)
}

func drawDrops(dc *gogui.DrawContext, s float32, n int, c gogui.Color, fall float32) {
	for i := range n {
		x := s * (0.28 + 0.44*float32(i)/float32(max(n-1, 1)))
		off := fall + float32(i%2)*0.5
		off -= float32(int(off))
		y := s * (0.62 + 0.22*off)
		dc.Line(x, y, x-s*0.04, y+s*0.12, c, s*0.05)
	}
}

func drawFlakes(dc *gogui.DrawContext, s float32, n int, c gogui.Color, fall, drift float32) {
	for i := range n {
		off := fall + float32(i)*0.37
		off -= float32(int(off))
		x := s*(0.3+0.4*float32(i)/float32(max(n-1, 1))) + drift*s*0.03
		y, r := s*(0.66+0.24*off), s*0.07
		for k := range 3 {
			a := float64(k) * math.Pi / 3
			dx, dy := float32(math.Cos(a))*r, float32(math.Sin(a))*r
			dc.Line(x-dx, y-dy, x+dx, y+dy, c, s*0.03)
		}
	}
}
