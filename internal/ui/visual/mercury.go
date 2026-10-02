package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

// Ported from anchorapp100/ha-visualisations src/13-mercury.js (MIT).

func init() { register(Mercury, func() Visual { return newMercury() }) }

type satellite struct {
	a, sp, r0           float64
	band                int
	wob, ws, o, x, y, r float64
}

type droplet struct{ x, y, vx, vy, r, t0 float64 }

type mercury struct {
	st stage
	fr framer
	r  rng

	sats       []satellite
	drops      []droplet
	tint       [3]float64
	core, spin float64
}

const mercuryFloor = 0.745

func newMercury() *mercury {
	v := &mercury{r: seeded(33), tint: [3]float64{110, 130, 255}, core: 0.07}
	for i := range 6 {
		sp := 0.22 + v.r.next()*0.3
		if i%2 == 1 {
			sp = -sp
		}
		v.sats = append(v.sats, satellite{a: float64(i)/6*2*math.Pi + v.r.next()*0.4, sp: sp, r0: 0.026 + v.r.next()*0.014, band: 2 + i*3,
			wob: v.r.next() * 2 * math.Pi, ws: 0.6 + v.r.next()*0.9, o: 0.16})
	}
	return v
}

type mercuryBall struct{ x, y, r float64 }

func (v *mercury) advance(f frame) (balls []mercuryBall, cx, cy float64) {
	t, dt, lv := f.t, f.dt, f.level
	talk := f.state == listening || f.state == responding
	tgt := [3]float64{110, 130, 255}
	switch f.state {
	case listening:
		tgt = [3]float64{60, 200, 255}
	case responding:
		tgt = [3]float64{255, 70, 200}
	}
	v.tint = mixc(v.tint, tgt, 1-math.Pow(0.94, dt*30))
	v.spin += dt * (0.35 + 1.6*lv)
	v.core = follow(v.core, 0.068+0.03*lv+0.012*f.slow, 0.3, 0.12, dt)

	cx, cy = 0.5+0.012*math.Sin(t*0.7), 0.42+0.01*math.Sin(t*0.9+1)
	balls = []mercuryBall{{cx, cy, v.core}}
	for i := range v.sats {
		sa := &v.sats[i]
		be := 0.0
		if talk {
			be = (f.bands[sa.band] + f.bands[sa.band+1]) * 0.5
		}
		oT := 0.12 + 0.03*math.Sin(t*0.45+float64(i)*1.7)
		if talk {
			oT = 0.12 + 0.15*lv + 0.05*be
		}
		sa.o = follow(sa.o, oT, 0.22, 0.1, dt)
		sa.a += dt * sa.sp * (0.5 + 1.8*lv)
		rT := sa.r0 * (0.9 + 0.12*math.Sin(t*sa.ws+sa.wob))
		if talk {
			rT = sa.r0 * (0.75 + 0.8*be)
		}
		if sa.r == 0 {
			sa.r = rT
		}
		sa.r = follow(sa.r, rT, 0.3, 0.1, dt)
		sa.x = cx + math.Cos(sa.a+v.spin*0.25)*sa.o*1.15 + 0.01*math.Sin(t*sa.ws+sa.wob)
		sa.y = cy + math.Sin(sa.a+v.spin*0.25)*sa.o*0.62
		balls = append(balls, mercuryBall{sa.x, sa.y, sa.r})
	}
	if f.onset && talk && len(v.drops) < 10 {
		nd := 1
		if f.voice > 0.55 {
			nd = 2
		}
		for range nd {
			ang := -math.Pi * (0.08 + 0.84*v.r.next())
			spd := 0.6 + 0.8*f.voice
			v.drops = append(v.drops, droplet{cx + math.Cos(ang)*v.core*0.7, cy + math.Sin(ang)*v.core*0.7, math.Cos(ang) * spd, math.Sin(ang) * spd, 0.018 + 0.016*f.voice, t})
		}
	}
	kept := v.drops[:0]
	for _, d := range v.drops {
		age := t - d.t0
		if age > 3.4 {
			continue
		}
		h := dt / 2
		for range 2 {
			d.vx += (-10*(d.x-cx) - 1.8*d.vx) * h
			d.vy += (-10*(d.y-cy) - 1.8*d.vy) * h
			d.x += d.vx * h
			d.y += d.vy * h
		}
		if d.y > mercuryFloor-d.r {
			d.y, d.vy = mercuryFloor-d.r, -math.Abs(d.vy)*0.3
		}
		rr := d.r
		if age > 2.6 {
			rr *= 1 - (age-2.6)/0.8
		}
		balls = append(balls, mercuryBall{d.x, d.y, rr})
		kept = append(kept, d)
	}
	v.drops = kept

	return balls, cx, cy
}

func (v *mercury) backdrop(back *image.RGBA, unit float64) {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	fy := mercuryFloor * unit
	fx.LinearRect(back, back.Bounds(), 0, 0, 0, float32(H), []fx.Stop{
		{At: 0, C: hex(0x040509, 1)}, {At: 0.55, C: hex(0x090b12, 1)}, {At: float32(fy/H) - 0.001, C: hex(0x11141d, 1)}, {At: float32(fy / H), C: hex(0x07080c, 1)}, {At: 1, C: hex(0x020203, 1)},
	}, false)
	fx.LinearRect(back, image.Rect(0, int(fy-0.6*u), v.st.w, int(fy-0.6*u)+max(1, int(1.2*u))), 0, 0, float32(W), 0, []fx.Stop{
		{At: 0, C: color.NRGBA{160, 175, 210, 0}}, {At: 0.5, C: color.NRGBA{160, 175, 210, 41}}, {At: 1, C: color.NRGBA{160, 175, 210, 0}},
	}, false)
	fx.Radial(back, float32(W/2), float32(H*0.45), 0, float32(math.Max(W, H)*0.75), []fx.Stop{{At: 0, C: color.NRGBA{}}, {At: 1, C: color.NRGBA{0, 0, 0, 178}}}, false)
}

//go:embed mercury.glsl
var mercuryShader string

func (v *mercury) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	resized := v.st.w != in.W || v.st.h != in.H
	if !v.st.measure(in) {
		return nil
	}
	W, H := float64(v.st.w), float64(v.st.h)
	unit := math.Min(H, W*1.3)
	ox := (W - unit) / 2
	if fresh {
		if err := g.Program(mercuryShader, Light|Splat); err != nil {
			return err
		}
	}
	if fresh || resized {
		back := image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
		v.backdrop(back, unit)
		if err := g.Texture(0, back); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	lv := f.level
	balls, cx, cy := v.advance(f)
	pts := make([]Point, 0, len(balls))
	for _, b := range balls {
		br := b.r * unit
		if br < 0.4 {
			continue
		}
		pts = append(pts, Point{X: float32(ox + b.x*unit), Y: float32(b.y * unit), Size: float32(br * 4.4), A: float32(br), Splat: true})
	}
	if err := g.Points(pts); err != nil {
		return err
	}
	tc := v.tint
	vals := []float32{float32(1 / W), float32(1 / H), float32(tc[0]), float32(tc[1]), float32(tc[2]), float32(lv),
		float32(ox + cx*unit), float32(cy * unit), float32(unit), float32(mercuryFloor * unit),
		float32(0.55 + 1.5*lv), float32(0.3 + 1.3*lv)}
	return g.Values(vals, float32(0.8+0.7*lv), 0.03, 2)
}
