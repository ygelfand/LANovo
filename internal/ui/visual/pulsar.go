package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Ported from anchorapp100/ha-visualisations src/17-pulsar.js (MIT).

func init() { register(Pulsar, func() Visual { return newPulsar() }) }

const (
	pulsarN     = 54
	pulsarP     = 80
	pulsarEvery = 0.055
)

type pulsar struct {
	st stage
	fr framer

	lines [][pulsarP]float64
	live  [pulsarP]float64
	acc   float64
	n     int
}

func newPulsar() *pulsar {
	v := &pulsar{lines: make([][pulsarP]float64, pulsarN)}
	for i := range pulsarN {
		pulsarFill(&v.lines[i], nil, float64(i)*7.13, 0, 0)
	}
	return v
}

func pulsarFill(arr *[pulsarP]float64, f *frame, seed, voice, lv float64) {
	for k := range pulsarP {
		xn := float64(k) / (pulsarP - 1)
		w := math.Exp(-math.Pow((xn-0.5)/0.2, 4))
		bi := math.Max(0, math.Min(1, (xn-0.24)/0.52)) * 26
		b0 := int(bi)
		bf := bi - float64(b0)
		bv := 0.0
		if f != nil {
			bv = f.bands[b0]*(1-bf) + f.bands[min(analysis.Bands-1, b0+1)]*bf
		}
		tex := noise2(float64(k)*0.55, seed) - 0.5
		hill := noise2(float64(k)*0.13+3, seed*0.37)
		arr[k] = math.Max(0, w*(0.12+0.22*hill+0.08*tex+(bv*1.25+0.22*tex*voice)*(0.25+1.05*lv))+0.018*tex+0.01)
	}
}

func (v *pulsar) advance(f frame) {
	talk := f.state == listening || f.state == responding
	vl, ll := 0.02, 0.05
	var src *frame
	if talk {
		vl, ll, src = f.voice, f.level, &f
	}
	v.acc += f.dt
	for v.acc >= pulsarEvery {
		v.acc -= pulsarEvery
		v.n++
		old := v.lines[0]
		copy(v.lines, v.lines[1:])
		pulsarFill(&old, src, float64(v.n)*7.13, vl, ll)
		v.lines[pulsarN-1] = old
	}
	pulsarFill(&v.live, src, float64(v.n+1)*7.13, vl, ll)
}

func (v *pulsar) geometry(portrait bool) (x0, pw, top, gap float64) {
	W, H := float64(v.st.w), float64(v.st.h)
	pw = math.Min(W*0.5, H*0.95)
	if portrait {
		pw = W * 0.82
	}
	return (W - pw) / 2, pw, H * 0.13, (H * 0.6) / pulsarN
}

//go:embed pulsar.glsl
var pulsarShader string

func (v *pulsar) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	resized := v.st.w != in.W || v.st.h != in.H
	if !v.st.measure(in) {
		return nil
	}
	u := v.st.u
	x0, pw, top, gap := v.geometry(Portrait(in))
	if fresh {
		if err := g.Program(pulsarShader, Light); err != nil {
			return err
		}
	}
	if fresh || resized {
		back := image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
		fx.Clear(back, color.NRGBA{0, 0, 0, 255})
		font := ui.MustLoad(ui.Bold, max(int(10*u*1.6), 7))
		label := "PSR B1919+21  ·  ASSIST"
		tw, _ := font.Measure(label)
		ui.DrawText(rgbaSurface{back}, font, int(x0+pw)-tw, int(top-34*u+gap), theme.Color{R: 90, G: 90, B: 90}, theme.Color{}, label)
		if err := g.Texture(0, back); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	v.advance(f)
	hts := image.NewRGBA(image.Rect(0, 0, pulsarP, pulsarN+2))
	for i := 0; i <= pulsarN; i++ {
		arr := &v.live
		if i < pulsarN {
			arr = &v.lines[i]
		}
		for k := range pulsarP {
			q := uint32(clamp01(arr[k]/2) * 65535)
			o := hts.PixOffset(k, i)
			hts.Pix[o], hts.Pix[o+1], hts.Pix[o+3] = uint8(q>>8), uint8(q), 255
		}
	}
	if err := g.Texture(1, hts); err != nil {
		return err
	}
	tint := [3]float64{235, 235, 235}
	switch f.state {
	case responding:
		tint = [3]float64{255, 150, 200}
	case listening:
		tint = [3]float64{140, 220, 255}
	}
	vals := []float32{float32(top), float32(gap), float32(gap * 7.5), float32(v.acc / pulsarEvery), float32(x0), float32(pw),
		float32(math.Max(1, 1.25*u)), float32(v.st.w), float32(tint[0] / 255), float32(tint[1] / 255), float32(tint[2] / 255)}
	return g.Values(vals, float32(0.22+0.25*f.level), 0.012, 2)
}
