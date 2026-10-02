package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"

	"github.com/tfriedel6/canvas"
	"github.com/tfriedel6/canvas/backend/softwarebackend"

	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

// Ported from anchorapp100/ha-visualisations src/19-tesla.js (MIT).

func init() { register(Tesla, func() Visual { return &tesla{r: seeded(55)} }) }

type filament struct{ lon, lat, sp, seed, on float64 }

type branch struct {
	t0, at, lon, lat, seed, len float64
	fi                          int
}

type tesla struct {
	st stage
	fr framer
	r  rng

	fil      []filament
	branches []branch
	flash    float64
	mix      float64

	w, h       int
	cx, cy, rg float64
	glass      *image.RGBA
	px, py     [65]float64
	fxs, fys   [65]float64
	paths      []teslaPath
	back       *image.RGBA
	ends       []teslaEnd
}

func (v *tesla) sky(back *image.RGBA) {
	H := float64(v.st.h)
	cx, cy := v.cx, v.cy

	fx.Clear(back, color.NRGBA{3, 2, 8, 255})
	fx.Radial(back, float32(cx), float32(cy), 0, float32(H*0.8), []fx.Stop{
		{At: 0, C: color.NRGBA{60, 30, 110, 77}}, {At: 1, C: color.NRGBA{}},
	}, false)
}

func (v *tesla) setup() {
	W, H := float64(v.st.w), float64(v.st.h)
	v.cx, v.cy, v.rg = W/2, H*0.4, math.Min(H*0.27, W*0.42)
	if !Portrait(ui.Rect{W: v.st.w, H: v.st.h}) {
		v.cy = H * 0.45
	}
	v.fil = v.fil[:0]
	for i := range 16 {
		on := 0.0
		if i < 5 {
			on = 1
		}
		v.fil = append(v.fil, filament{lon: v.r.next() * 2 * math.Pi, lat: (v.r.next() - 0.5) * 2.4, sp: 0.15 + v.r.next()*0.3, seed: v.r.next() * 100, on: on})
	}

	cx, cy, rg, u := v.cx, v.cy, v.rg, v.st.u

	v.back = image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
	v.sky(v.back)

	be := softwarebackend.New(v.st.w, v.st.h)
	cv := canvas.New(be)
	rim := cv.CreateRadialGradient(cx, cy, rg*0.86, cx, cy, rg*1.01)
	rim.AddColorStop(0, color.NRGBA{180, 170, 255, 0})
	rim.AddColorStop(0.85, color.NRGBA{190, 180, 255, 41})
	rim.AddColorStop(1, color.NRGBA{220, 215, 255, 87})
	cv.SetFillStyle(rim)
	cv.BeginPath()
	cv.Arc(cx, cy, rg, 0, 2*math.Pi, false)
	cv.Fill()
	cv.Save()
	cv.Translate(cx-rg*0.42, cy-rg*0.5)
	cv.Rotate(-0.7)
	cv.Scale(1, 0.55)
	hl := cv.CreateRadialGradient(0, 0, 0, 0, 0, rg*0.3)
	hl.AddColorStop(0, color.NRGBA{255, 255, 255, 107})
	hl.AddColorStop(1, color.NRGBA{255, 255, 255, 0})
	cv.SetFillStyle(hl)
	cv.BeginPath()
	cv.Arc(0, 0, rg*0.3, 0, 2*math.Pi, false)
	cv.Fill()
	cv.Restore()
	cv.SetStrokeStyle(color.NRGBA{230, 225, 255, 20})
	cv.SetLineWidth(2 * u)
	cv.BeginPath()
	cv.Arc(cx, cy, rg*0.93, 0.3, 1.25, false)
	cv.Stroke()
	v.glass = be.Image
	v.w, v.h = v.st.w, v.st.h
}

func (v *tesla) arc(x0, y0, x1, y1, seed, rough, t, lv float64) {
	const n = 64
	v.px[0], v.py[0], v.px[n], v.py[n] = x0, y0, x1, y1
	for step := n; step > 1; step >>= 1 {
		for a := 0; a < n; a += step {
			b, m := a+step, a+step/2
			dx, dy := v.px[b]-v.px[a], v.py[b]-v.py[a]
			l := math.Sqrt(dx*dx + dy*dy)
			j := (noise1(seed+float64(m)*0.37+t*(3.5+6*lv))-0.5)*2 + (v.r.next()-0.5)*0.35
			v.px[m] = (v.px[a]+v.px[b])/2 - dy/(l+1e-6)*l*rough*j
			v.py[m] = (v.py[a]+v.py[b])/2 + dx/(l+1e-6)*l*rough*j
		}
		rough *= 0.62
	}
}

type teslaPath struct {
	x, y          [65]float64
	bright, width float64
}

type teslaEnd struct{ x, y, bright float64 }

func (v *tesla) advance(f frame) (cA, cB [3]float64) {
	t, dt, lv := f.t, f.dt, f.level
	cx, cy, rg := v.cx, v.cy, v.rg
	talk := f.state == listening || f.state == responding
	target := v.mix
	switch f.state {
	case responding:
		target = 1
	case listening:
		target = 0
	}
	v.mix = follow(v.mix, target, 0.08, 0.08, dt)
	nf := 5
	if talk {
		nf = 5 + int(math.Round(11*math.Min(1, lv*1.3)))
	}
	cA = mixc([3]float64{90, 150, 255}, [3]float64{255, 70, 200}, v.mix)
	cB = mixc([3]float64{150, 100, 255}, [3]float64{180, 90, 255}, v.mix)
	if f.onset && talk {
		v.flash = 1
		for range 3 + int(f.voice*4) {
			v.branches = append(v.branches, branch{t0: t, fi: int(v.r.next() * float64(nf)), at: 0.35 + v.r.next()*0.5,
				lon: v.r.next() * 2 * math.Pi, lat: (v.r.next() - 0.5) * 2.6, seed: v.r.next() * 99, len: 0.25 + 0.35*v.r.next()})
		}
	}
	v.flash = math.Max(0, v.flash-dt*4)
	live := v.branches[:0]
	for _, b := range v.branches {
		if t-b.t0 < 0.28 {
			live = append(live, b)
		}
	}
	v.branches = live

	rough := 0.2 + 0.12*lv + 0.1*v.flash
	coreR := rg * 0.075
	v.paths, v.ends = v.paths[:0], v.ends[:0]
	for i := range v.fil {
		fl := &v.fil[i]
		on := 0.0
		if i < nf {
			on = 1
		}
		fl.on = follow(fl.on, on, 0.25, 0.12, dt)
		if fl.on < 0.03 {
			continue
		}
		dir := 1.0
		if i%2 == 1 {
			dir = -1
		}
		fl.lon += dt * fl.sp * (0.35 + 1.2*lv) * dir
		lat := fl.lat*0.5 + 0.35*math.Sin(t*fl.sp+fl.seed)
		vx, vy, vz := math.Cos(lat)*math.Sin(fl.lon), math.Sin(lat), math.Cos(lat)*math.Cos(fl.lon)
		ex, ey := cx+vx*rg*0.97, cy-vy*rg*0.97
		bright := fl.on * (0.45 + 0.55*(vz*0.5+0.5)) * (0.7 + 0.5*lv + 0.6*v.flash)
		v.arc(cx+vx*coreR, cy-vy*coreR, ex, ey, fl.seed, rough, t, lv)
		v.paths = append(v.paths, teslaPath{x: v.px, y: v.py, bright: bright, width: 0.8 + 0.5*lv})
		v.ends = append(v.ends, teslaEnd{ex, ey, bright})
		v.fxs, v.fys = v.px, v.py
		for _, b := range v.branches {
			if b.fi != i {
				continue
			}
			ai := int(math.Round(b.at * 64))
			bx, by := v.fxs[ai], v.fys[ai]
			ag := (t - b.t0) / 0.28
			bl, bu, l2 := math.Cos(b.lat)*math.Sin(b.lon), math.Sin(b.lat), rg*b.len
			v.arc(bx, by, bx+bl*l2, by-bu*l2, b.seed, 0.35, t, lv)
			v.paths = append(v.paths, teslaPath{x: v.px, y: v.py, bright: (1 - ag) * 0.9, width: 0.6})
		}
	}
	return cA, cB
}

//go:embed tesla.glsl
var teslaShader string

func (v *tesla) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(teslaShader, Light|Lines); err != nil {
			return err
		}
	}
	if fresh || v.back == nil || v.w != v.st.w || v.h != v.st.h {
		v.setup()
		if err := g.Texture(0, v.back); err != nil {
			return err
		}
		if err := g.Texture(1, v.glass); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	lv := f.level
	cA, cB := v.advance(f)
	u := v.st.u
	core := mixc(cA, [3]float64{255, 255, 255}, 0.7)
	segs := make([]Segment, 0, len(v.paths)*64*3)
	for _, pa := range v.paths {
		for _, layer := range []struct {
			w, a float64
			c    [3]float64
		}{{10, 0.07, cB}, {3.6, 0.32, cA}, {1.3, 0.85, core}} {
			a := float32(math.Min(layer.a*pa.bright, 1))
			for i := range 64 {
				segs = append(segs, Segment{X0: float32(pa.x[i]), Y0: float32(pa.y[i]), X1: float32(pa.x[i+1]), Y1: float32(pa.y[i+1]),
					Width: float32(layer.w * u * pa.width), R: float32(layer.c[0] / 255), G: float32(layer.c[1] / 255), B: float32(layer.c[2] / 255), A: a})
			}
		}
	}
	if err := g.Lines(segs); err != nil {
		return err
	}
	vals := make([]float32, 66)
	for k := range 3 {
		vals[k], vals[3+k] = float32(cA[k]/255), float32(cB[k]/255)
	}
	vals[6], vals[7], vals[8], vals[9], vals[10] = float32(v.cx), float32(v.cy), float32(v.rg), float32(lv), float32(v.flash)
	vals[11] = float32(v.rg * 0.075 * (1.1 + 0.5*lv + 0.9*v.flash))
	vals[14], vals[15] = float32(v.st.w), float32(v.st.h)
	tip := mixc(cA, [3]float64{255, 255, 255}, 0.6)
	pts := make([]Point, 0, len(v.ends))
	for _, e := range v.ends {
		pts = append(pts, Point{X: float32(e.x), Y: float32(e.y), Size: float32(2 * v.rg * (0.05 + 0.04*lv)),
			R: float32(tip[0] / 255), G: float32(tip[1] / 255), B: float32(tip[2] / 255), A: float32(math.Min(0.8*e.bright, 1)), Round: true, Light: true})
	}
	if err := g.Points(pts); err != nil {
		return err
	}
	return g.Values(vals, float32(0.75+0.7*lv+0.5*v.flash), 0.025, 2)
}
