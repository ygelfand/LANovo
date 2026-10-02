package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

// Ported from anchorapp100/ha-visualisations src/23-fireworks.js (MIT).

func init() { register(Fireworks, func() Visual { return &fireworks{r: seeded(404)} }) }

const fwMax = 1800

var (
	fwPA = [4][3]float64{{120, 210, 255}, {70, 130, 255}, {140, 255, 220}, {235, 245, 255}}
	fwPB = [4][3]float64{{255, 95, 205}, {255, 70, 120}, {255, 205, 100}, {200, 130, 255}}
	fwPG = [4][3]float64{{255, 205, 95}, {255, 165, 60}, {255, 235, 170}, {255, 190, 110}}
)

type fwRocket struct {
	x, y, vx, vy, str float64
	kind              int
	ci                int
}

type fwSpark struct {
	x, y, vx, vy  float64
	c             [3]float64
	t0, life      float64
	drag, grav, s float64
	twinkle       bool
}

type fwFlash struct {
	x, y, t0, a float64
	c           [3]float64
}

type fireworks struct {
	st stage
	fr framer
	r  rng

	w, h     int
	ground   float64
	skyTop   int
	sky      *image.RGBA
	acc      *image.RGBA
	rockets  []fwRocket
	sparks   []fwSpark
	flashes  []fwFlash
	nextAuto float64
	mix      float64
	marks    []fwMark
	back     *image.RGBA
}

func (v *fireworks) stars(back *image.RGBA, r *rng) {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	fx.LinearRect(back, back.Bounds(), 0, 0, 0, float32(H), []fx.Stop{
		{At: 0, C: hex(0x010208, 1)}, {At: 0.55, C: hex(0x050a1a, 1)}, {At: 0.86, C: hex(0x0d1430, 1)}, {At: 1, C: hex(0x070a16, 1)},
	}, false)
	for range 260 {
		b := 0.15 + r.next()*r.next()*0.7
		s := (0.5 + r.next()) * u
		fx.Square(back, float32(r.next()*W), float32(r.next()*H*0.7), float32(s), color.NRGBA{220, 230, 255, uint8(b * 255)}, false)
	}
}

func (v *fireworks) setup() {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	r := seeded(404)
	v.ground = H * 0.86
	v.back = image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
	v.stars(v.back, &r)

	v.sky = image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
	put := func(x, y, w, h float64, c color.NRGBA) {
		for py := max(int(y), 0); py < min(int(math.Ceil(y+h)), v.st.h); py++ {
			for px := max(int(x), 0); px < min(int(math.Ceil(x+w)), v.st.w); px++ {
				i := py*v.sky.Stride + px*4
				a := uint32(c.A)
				inv := 255 - a
				v.sky.Pix[i] = uint8((uint32(c.R)*a + uint32(v.sky.Pix[i])*inv) / 255)
				v.sky.Pix[i+1] = uint8((uint32(c.G)*a + uint32(v.sky.Pix[i+1])*inv) / 255)
				v.sky.Pix[i+2] = uint8((uint32(c.B)*a + uint32(v.sky.Pix[i+2])*inv) / 255)
				v.sky.Pix[i+3] = uint8(a + uint32(v.sky.Pix[i+3])*inv/255)
			}
		}
	}
	body := color.NRGBA{3, 4, 10, 255}
	top := v.ground
	var wins [][2]float64
	for x := 0.0; x < W; {
		bw, bh := (26+r.next()*70)*u, H*(0.05+r.next()*r.next()*0.13)
		t := v.ground - bh
		put(x, t, bw+1, H-t, body)
		if r.next() < 0.25 {
			put(x+bw*0.4, t-bh*0.25, bw*0.12, bh*0.25, body)
			top = math.Min(top, t-bh*0.25)
		}
		top = math.Min(top, t)
		for wy := t + 6*u; wy < v.ground-4*u; wy += 9 * u {
			for wx := x + 5*u; wx < x+bw-6*u; wx += 8 * u {
				if r.next() < 0.16 {
					wins = append(wins, [2]float64{wx, wy})
				}
			}
		}
		x += bw
		if r.next() < 0.3 {
			x += r.next() * 16 * u
		}
	}
	for _, w := range wins {
		put(w[0], w[1], 3*u, 3.4*u, color.NRGBA{255, 200, 110, 140})
	}
	v.skyTop = max(int(top), 0)
	v.acc = image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
	v.rockets, v.sparks, v.flashes = v.rockets[:0], v.sparks[:0], v.flashes[:0]
	v.w, v.h = v.st.w, v.st.h
}

func (v *fireworks) launch(str float64) {
	W, H := float64(v.st.w), float64(v.st.h)
	yb := H*(0.44-0.26*str) + (v.r.next()-0.5)*H*0.06
	gy := 0.55 * H
	vy := -math.Sqrt(2 * gy * math.Max(10, v.ground-yb))
	x := W * (0.2 + 0.6*v.r.next())
	p := v.r.next()
	kind := 0
	if str > 0.7 && p < 0.3 {
		kind = 1
	} else if p < 0.22 {
		kind = 2
	}
	v.rockets = append(v.rockets, fwRocket{x: x, y: v.ground, vx: (v.r.next() - 0.5) * W * 0.05, vy: vy, str: str, kind: kind, ci: int(v.r.next() * 4)})
}

func (v *fireworks) burst(rk fwRocket, cols [4][3]float64, t float64) {
	H := float64(v.st.h)
	n := int(math.Round(60 + 120*rk.str))
	sp := H * (0.2 + 0.24*rk.str)
	c, c2 := cols[rk.ci], cols[(rk.ci+1)%4]
	for i := 0; i < n && len(v.sparks) < fwMax; i++ {
		a := v.r.next() * 2 * math.Pi
		vel := sp * (0.35 + 0.65*math.Sqrt(v.r.next()))
		if rk.kind == 1 {
			vel = sp
		}
		s := fwSpark{t0: t, drag: 1.35, grav: 0.28, s: 1.4, twinkle: v.r.next() < 0.45}
		if rk.kind == 2 {
			vel *= 0.7
			s.c = fwPG[int(v.r.next()*2)]
			s.life, s.drag, s.grav, s.s = 2.4+v.r.next()*0.8, 2.4, 0.5, 1
		} else {
			s.c = c
			if v.r.next() >= 0.8 {
				s.c = c2
			}
			s.life = 1.1 + v.r.next()*0.8
		}
		s.x, s.y, s.vx, s.vy = rk.x, rk.y, math.Cos(a)*vel, math.Sin(a)*vel
		v.sparks = append(v.sparks, s)
	}
	v.flashes = append(v.flashes, fwFlash{x: rk.x, y: rk.y, t0: t, a: 0.1 + 0.2*rk.str, c: c})
}

type fwMark struct {
	x0, y0, x, y, size, a float64
	c                     [3]float64
	line                  bool
}

func (v *fireworks) advance(f frame) (cols [4][3]float64) {
	H, u := float64(v.st.h), v.st.u
	t, dt, lv := f.t, f.dt, f.level
	talk := f.state == listening || f.state == responding
	switch f.state {
	case responding:
		v.mix = follow(v.mix, 1, 0.08, 0.08, dt)
	case listening:
		v.mix = follow(v.mix, 0, 0.08, 0.08, dt)
	}
	for k := range cols {
		cols[k] = mixc(fwPA[k], fwPB[k], v.mix)
	}
	if talk && f.onset && len(v.rockets) < 8 {
		v.launch(0.4 + 0.6*f.voice)
	}
	if t > v.nextAuto || v.nextAuto-t > 10 {
		if talk {
			v.nextAuto = t + 0.9 - 0.55*lv
			if lv > 0.2 {
				v.launch(0.3 + 0.5*lv)
			}
		} else {
			v.nextAuto = t + 2.6 + 2*v.r.next()
			v.launch(0.3)
		}
	}
	v.marks = v.marks[:0]
	gy := 0.55 * H
	kept := v.rockets[:0]
	for _, rk := range v.rockets {
		px, py := rk.x, rk.y
		rk.vy += gy * dt
		rk.x += rk.vx * dt
		rk.y += rk.vy * dt
		v.marks = append(v.marks, fwMark{x0: px, y0: py, x: rk.x, y: rk.y, size: 1.6 * u, a: 0.6, c: [3]float64{255, 205, 140}, line: true})
		if v.r.next() < 0.8 && len(v.sparks) < fwMax {
			v.sparks = append(v.sparks, fwSpark{x: rk.x, y: rk.y, vx: (v.r.next() - 0.5) * 30 * u, vy: 40 * u, c: [3]float64{255, 220, 170}, t0: t, life: 0.35, drag: 3, grav: 0.2, s: 0.8})
		}
		if rk.vy > -0.06*H {
			v.burst(rk, cols, t)
			continue
		}
		kept = append(kept, rk)
	}
	v.rockets = kept

	alv := [3]float64{0.3, 0.62, 1}
	live := v.sparks[:0]
	for _, p := range v.sparks {
		age := t - p.t0
		if age > p.life || age < 0 {
			continue
		}
		d := math.Exp(-p.drag * dt)
		p.vx *= d
		p.vy *= d
		p.vy += gy * p.grav * dt
		ox, oy := p.x, p.y
		p.x += p.vx * dt
		p.y += p.vy * dt
		fade := 1 - age/p.life
		al := fade * fade
		if p.twinkle && fade < 0.45 && v.r.next() < 0.5 {
			al *= 0.2
		}
		lvl := 0
		if al > 0.66 {
			lvl = 2
		} else if al > 0.3 {
			lvl = 1
		}
		sz := p.s * (1.2 + 1.2*fade) * u
		c := mixc(p.c, [3]float64{255, 255, 255}, float64(lvl)*0.18)
		if math.Hypot(p.x-ox, p.y-oy) > sz {
			v.marks = append(v.marks, fwMark{x0: ox, y0: oy, x: p.x, y: p.y, size: math.Min(1, sz), a: alv[lvl] * math.Min(1, sz), c: c, line: true})
		}
		v.marks = append(v.marks, fwMark{x: p.x, y: p.y, size: sz, a: alv[lvl], c: c})
		live = append(live, p)
	}
	v.sparks = live
	return cols
}

//go:embed fireworks.glsl
var fireworksShader string

func (v *fireworks) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(fireworksShader, Light|Feed); err != nil {
			return err
		}
	}
	if fresh || v.back == nil || v.w != v.st.w || v.h != v.st.h {
		v.setup()
		if err := g.Texture(0, v.back); err != nil {
			return err
		}
		if err := g.Texture(1, v.sky); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	W, H := float64(v.st.w), float64(v.st.h)
	t, lv := f.t, f.level
	cols := v.advance(f)

	pts := make([]Point, 0, len(v.marks)*2)
	add := func(x, y, size float64, c [3]float64, a float64) {
		pts = append(pts, Point{X: float32(x), Y: float32(y), Size: float32(math.Max(size, 1)),
			R: float32(c[0] / 255), G: float32(c[1] / 255), B: float32(c[2] / 255), A: float32(a), Feed: true})
	}
	for _, m := range v.marks {
		if !m.line {
			add(m.x, m.y, m.size, m.c, m.a)
			continue
		}
		n := min(int(math.Hypot(m.x-m.x0, m.y-m.y0)/math.Max(m.size, 1))+1, 24)
		for k := range n {
			q := float64(k) / float64(n)
			add(m.x0+(m.x-m.x0)*q, m.y0+(m.y-m.y0)*q, m.size, m.c, m.a)
		}
	}
	if err := g.Points(pts); err != nil {
		return err
	}

	vals := make([]float32, 26)
	vals[0] = float32(math.Pow(0.84, f.dt*30))
	flashes := v.flashes[:0]
	for _, fl := range v.flashes {
		if fa := 1 - (t-fl.t0)/0.45; fa > 0 && fa <= 1 {
			flashes = append(flashes, fl)
		}
	}
	v.flashes = flashes
	sky := 0.0
	for i, fl := range v.flashes[max(0, len(v.flashes)-3):] {
		fa := 1 - (t-fl.t0)/0.45
		sky = math.Max(sky, fa*fl.a)
		b := 1 + 6*i
		vals[b], vals[b+1], vals[b+2] = float32(fl.x), float32(fl.y), float32(fl.a*fa)
		vals[b+3], vals[b+4], vals[b+5] = float32(fl.c[0]/255), float32(fl.c[1]/255), float32(fl.c[2]/255)
	}
	vals[19], vals[20] = float32(v.ground), float32(sky)
	for k := range 3 {
		vals[21+k] = float32(cols[0][k] / 255)
	}
	vals[24], vals[25] = float32(W), float32(H)
	return g.Values(vals, float32(0.85+0.5*lv), 0.022, 2)
}
