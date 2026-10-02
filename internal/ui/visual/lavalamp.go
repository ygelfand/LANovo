package visual

import (
	_ "embed"
	"image"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
)

func init() { register(LavaLamp, func() Visual { return &lavaLamp{r: seeded(1963)} }) }

const lavaBlobs = 10

type lavaBlob struct{ x, y, vy, r, heat float64 }

type lavaLamp struct {
	st stage
	fr framer
	r  rng

	w, h  int
	blobs [lavaBlobs]lavaBlob
	speed float64
	glow  float64
	born  bool
}

func (v *lavaLamp) glass() (cx, top, bottom, neck, belly float64) {
	W, H := float64(v.st.w), float64(v.st.h)
	gh := H * 0.9 / (1 + lavaCap + lavaBase)
	top = H*0.05 + gh*lavaCap
	bottom = top + gh
	belly = math.Min(W*0.18, gh*0.19)
	return W / 2, top, bottom, belly * 0.36, belly
}

const (
	lavaCap   = 0.13
	lavaBase  = 0.44
	lavaWaist = 0.62
	lavaWide  = 0.26
)

func (v *lavaLamp) halfWidth(y float64) float64 {
	_, top, bottom, neck, belly := v.glass()
	s := clamp01((bottom - y) / (bottom - top))
	if s < lavaWide {
		return belly * (lavaWaist + (1-lavaWaist)*math.Sin(s/lavaWide*math.Pi/2))
	}
	return belly + (neck-belly)*(s-lavaWide)/(1-lavaWide)
}

func (v *lavaLamp) paint() *image.RGBA {
	w, h := max(v.st.w/2, 1), max(v.st.h/2, 1)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	cx, top, bottom, neck, belly := v.glass()
	W, H := float64(v.st.w), float64(v.st.h)
	gh := bottom - top
	capTop, baseBot, lip := top-gh*lavaCap, bottom+gh*lavaBase, gh*0.025
	for y := range h {
		for x := range w {
			fx, fy := (float64(x)+0.5)*2, (float64(y)+0.5)*2
			dx, dy := (fx-cx)/W, (fy-(top+bottom)/2)/H
			room := 0.03 + 0.07*math.Exp(-(dx*dx*6+dy*dy*3))
			r, g, b, a := room*1.1, room*0.7, room*0.9, 0.0
			switch {
			case fy >= top && fy <= bottom && math.Abs(fx-cx) <= v.halfWidth(fy):
				hw := v.halfWidth(fy)
				k := (fx - cx) / hw
				hi := math.Exp(-math.Pow((k+0.55)/0.12, 2))*0.35 + math.Exp(-math.Pow((k-0.62)/0.06, 2))*0.12
				r, g, b = hi, hi, hi*1.05
				a = clamp01((hw - math.Abs(fx-cx)) * 0.5)
			case fy > bottom-lip && fy <= baseBot:
				t := clamp01((fy - bottom - lip) / (baseBot - bottom - lip))
				hw := belly * (lavaWaist*1.08 + (1.3-lavaWaist*1.08)*math.Pow(t, 1.25))
				shade := 1.0
				if fy <= bottom+lip {
					hw, shade = belly*lavaWaist*1.1, 1.25
				}
				if math.Abs(fx-cx) <= hw {
					k := (fx - cx) / hw
					m := (0.3 + 0.5*math.Exp(-math.Pow((k+0.4)/0.22, 2)) + 0.15*math.Exp(-math.Pow((k-0.55)/0.1, 2)) - 0.2*k*k) * shade
					r, g, b = m*0.95, m*0.9, m*0.85
				}
			case fy >= capTop && fy < top:
				t := (fy - capTop) / (top - capTop)
				hw := neck * (0.55 + 0.45*t)
				if math.Abs(fx-cx) <= hw {
					k := (fx - cx) / hw
					m := 0.3 + 0.5*math.Exp(-math.Pow((k+0.35)/0.25, 2)) - 0.18*k*k
					r, g, b = m*0.95, m*0.9, m*0.85
				}
			}
			o := img.PixOffset(x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = uint8(255*math.Min(1, r)), uint8(255*math.Min(1, g)), uint8(255*math.Min(1, b)), uint8(255*a)
		}
	}
	return img
}

func (v *lavaLamp) spawn() {
	_, top, bottom, _, belly := v.glass()
	for i := range v.blobs {
		b := &v.blobs[i]
		b.y = top + (bottom-top)*(0.15+0.8*v.r.next())
		b.x = (v.r.next()*2 - 1) * v.halfWidth(b.y) * 0.5
		b.r = belly * (0.2 + 0.22*v.r.next())
		b.heat = v.r.next()
	}
	v.blobs[0].r, v.blobs[0].y = belly*0.5, bottom-belly*0.2
	v.born = true
}

func (v *lavaLamp) advance(f frame) {
	dt := f.dt
	_, top, bottom, _, _ := v.glass()
	talk := f.state == listening || f.state == responding
	want, bright := 1.0, 0.45
	if talk {
		want, bright = 1+4*f.level, 0.45+0.9*f.level
	}
	v.speed = follow(v.speed, want, 0.6, 0.25, dt)
	v.glow = follow(v.glow, bright, 0.6, 0.15, dt)
	gh := bottom - top
	for i := range v.blobs {
		b := &v.blobs[i]
		t := (b.y - top) / gh
		b.heat += dt * v.speed * (0.18*(t-0.55) + 0.02*(v.r.next()-0.5))
		b.heat = clamp01(b.heat)
		if f.onset && talk && t > 0.6 && v.r.next() < 0.5 {
			b.vy -= gh * 0.12
			b.heat = 1
		}
		b.vy += (0.5 - b.heat) * gh * 0.02 * dt * v.speed
		b.vy *= math.Pow(0.55, dt)
		b.y += b.vy * dt
		if i == 0 {
			b.y = math.Max(b.y, bottom-b.r*0.6)
		}
		b.y = math.Max(top+b.r*0.4, math.Min(b.y, bottom-b.r*0.3))
		lim := math.Max(0, v.halfWidth(b.y)-b.r*0.6)
		b.x += (v.r.next() - 0.5) * gh * 0.01 * dt * v.speed
		b.x = math.Max(-lim, math.Min(b.x, lim))
	}
}

//go:embed lavalamp.glsl
var lavaLampShader string

func (v *lavaLamp) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(lavaLampShader, Light); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		if err := g.Texture(0, v.paint()); err != nil {
			return err
		}
		v.w, v.h = v.st.w, v.st.h
		v.spawn()
	}
	f := v.fr.next(x)
	v.advance(f)
	cx, top, bottom, _, _ := v.glass()
	vals := make([]float32, 8+3*lavaBlobs)
	vals[0], vals[1], vals[2], vals[3], vals[4] = float32(v.st.w), float32(top), float32(bottom), float32(v.glow), float32(cx)
	for i, b := range v.blobs {
		o := 8 + 3*i
		vals[o], vals[o+1], vals[o+2] = float32(cx+b.x), float32(b.y), float32(b.r*b.r)
	}
	return g.Values(vals, float32(0.35+0.3*v.glow), 0.03, 2)
}
