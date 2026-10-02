package visual

import (
	_ "embed"
	"image"
	"math"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

// Ported from anchorapp100/ha-visualisations src/24-synthwave.js (MIT).

func init() { register(Synthwave, func() Visual { return newSynthwave() }) }

const synthP = 72

type wave struct{ z, a float64 }

type synthwave struct {
	st stage
	fr framer

	scroll, mix       float64
	waves             []wave
	front, back, base [synthP]float64
}

func newSynthwave() *synthwave {
	v := &synthwave{}
	for i := range synthP {
		xn := float64(i) / (synthP - 1)
		v.base[i] = 0.25 + 0.5*noise1(xn*7+3) + 0.25*noise1(xn*19+1)
	}
	return v
}

func (v *synthwave) advance(f frame) (neon, neon2 [3]float64) {
	dt, lv := f.dt, f.level
	talk := f.state == listening || f.state == responding
	target := v.mix
	switch f.state {
	case responding:
		target = 1
	case listening:
		target = 0
	}
	v.mix = follow(v.mix, target, 0.08, 0.08, dt)
	neon = mixc([3]float64{60, 235, 255}, [3]float64{255, 60, 200}, v.mix)
	neon2 = mixc([3]float64{255, 60, 200}, [3]float64{80, 220, 255}, v.mix)
	v.scroll += dt * (0.9 + 3.6*lv)
	if f.onset && talk && len(v.waves) < 6 {
		v.waves = append(v.waves, wave{14, 0.5 + 0.5*f.voice})
	}
	for i := range synthP {
		xn := float64(i) / (synthP - 1)
		d := math.Abs(xn-0.5) * 2
		bi := d * 26
		b0 := int(bi)
		bv := f.bands[b0] + (f.bands[min(analysis.Bands-1, b0+1)]-f.bands[b0])*(bi-float64(b0))
		gap := smoothstep(0.1, 0.34, d)
		live := 0.08
		if talk {
			live = bv * (0.6 + 0.8*lv)
		}
		tgt := (v.base[i]*0.35 + live) * gap
		v.front[i] = follow(v.front[i], tgt, 0.45, 0.12, dt)
		v.back[i] = follow(v.back[i], v.base[i]*0.55*gap+tgt*0.35, 0.05, 0.03, dt)
	}

	return neon, neon2
}

func (v *synthwave) backdrop(back *image.RGBA) {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	hz := H * 0.6
	fx.LinearRect(back, image.Rect(0, 0, v.st.w, int(hz)), 0, 0, 0, float32(hz), []fx.Stop{
		{At: 0, C: hex(0x060217, 1)}, {At: 0.5, C: hex(0x1a0736, 1)}, {At: 0.82, C: hex(0x46104e, 1)}, {At: 1, C: hex(0x8c1f5c, 1)},
	}, false)
	r := seeded(88)
	for range 190 {
		b := 0.2 + r.next()*r.next()*0.8
		sz := (0.5 + r.next()*1.1) * u
		fx.Square(back, float32(r.next()*W), float32(math.Pow(r.next(), 1.6)*hz*0.7), float32(math.Max(sz, 0.6)), nrgba([3]float64{255, 230, 255}, b), false)
	}
	fx.LinearRect(back, image.Rect(0, int(hz), v.st.w, v.st.h), 0, float32(hz), 0, float32(H), []fx.Stop{
		{At: 0, C: hex(0x1a0530, 1)}, {At: 1, C: hex(0x050109, 1)},
	}, false)
}

func (v *synthwave) groundLines(back *image.RGBA) {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	hz := H * 0.6
	camH := H - hz
	cx := W / 2
	lw := 1.4 * u
	haze := [3]float64{26, 5, 48}
	for y := int(hz) + 1; y < v.st.h; y++ {
		py := float64(y) + 0.5
		dy := py - hz
		s := dy / (H*1.35 - hz)
		spread := W * 0.16 * (0.02 + 2.18*s)
		ht := dy / (camH * 0.35)
		keep := 1.0
		if ht < 1 && dy > 0.8*u {
			keep = 1 - 0.9*(1-ht)
		}
		for x := range v.st.w {
			px := float64(x) + 0.5
			kc := (px - cx) / spread
			kr := math.Floor(kc + 0.5)
			line := 0.0
			if math.Abs(kr) <= 26 {
				slope := kr * W * 0.16 * 2.18 / (H*1.35 - hz)
				d := math.Abs(kc-kr) * spread / math.Sqrt(1+slope*slope)
				line = clamp01(lw*0.5 - d + 0.5)
			}
			i := back.PixOffset(x, y)
			for c := range 3 {
				back.Pix[i+c] = uint8(float64(back.Pix[i+c])*keep + haze[c]*(1-keep))
			}
			back.Pix[i+3] = uint8(line * keep * 255)
		}
	}
}

//go:embed synthwave.glsl
var synthwaveShader string

func (v *synthwave) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	resized := v.st.w != in.W || v.st.h != in.H
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(synthwaveShader, Light); err != nil {
			return err
		}
	}
	if fresh || resized {
		back := image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
		v.backdrop(back)
		v.groundLines(back)
		if err := g.Texture(0, back); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	dt, lv := f.dt, f.level
	talk := f.state == listening || f.state == responding
	neon, neon2 := v.advance(f)
	hz := H * 0.6

	ridges := image.NewRGBA(image.Rect(0, 0, 256, 2))
	for row, arr := range []*[synthP]float64{&v.front, &v.back} {
		for i := range synthP {
			q := uint32(clamp01(arr[i]/2) * 65535)
			o := ridges.PixOffset(i, row)
			ridges.Pix[o], ridges.Pix[o+1], ridges.Pix[o+3] = uint8(q>>8), uint8(q), 255
		}
	}
	if err := g.Texture(1, ridges); err != nil {
		return err
	}

	bass := (f.bands[1] + f.bands[3] + f.bands[5]) / 3
	rs := math.Min(H, W*1.2) * (0.155 + 0.012*f.slow)
	stripe := 0.8 + 1.4*0.15
	if talk {
		rs += math.Min(H, W*1.2) * 0.012 * lv
		stripe = 0.8 + 1.4*bass
	}
	vals := make([]float32, 32)
	vals[0], vals[1], vals[2], vals[3] = float32(hz), float32(W/2), float32(hz-H*0.075), float32(rs)
	vals[4], vals[5] = float32(lv), float32(u)
	for k := range 3 {
		vals[6+k], vals[9+k] = float32(neon[k]/255), float32(neon2[k]/255)
	}
	vals[12], vals[13] = float32(stripe), float32(math.Mod(v.scroll, 1))
	vals[14], vals[15] = float32(W), float32(H)
	camH := H - hz
	kept := v.waves[:0]
	for _, w := range v.waves {
		w.z -= dt * (5 + 6*lv)
		if w.z < 0.9 {
			continue
		}
		if i := len(kept); i < 6 {
			vals[20+2*i] = float32(hz + camH*(1/math.Max(0.05, w.z*0.55))*0.55)
			vals[21+2*i] = float32(w.a * math.Min(1, (14-w.z)/3))
		}
		kept = append(kept, w)
	}
	v.waves = kept
	return g.Values(vals, float32(0.55+0.45*lv), 0.018, 2)
}
