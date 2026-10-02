package visual

import (
	_ "embed"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
)

// Ported from anchorapp100/ha-visualisations src/22-paint-splash.js (MIT).

func init() { register(PaintSplash, func() Visual { return &paint{r: seeded(71)} }) }

var (
	paintCool = [][3]float64{{25, 195, 255}, {30, 105, 255}, {70, 222, 45}, {255, 226, 28}, {20, 226, 190}}
	paintHot  = [][3]float64{{255, 40, 205}, {255, 52, 30}, {255, 138, 22}, {255, 208, 30}, {255, 88, 150}}
	paintAll  = append(append([][3]float64{}, paintCool...), paintHot...)
)

type paintBlob struct {
	x, y, vx, vy, r, r0 float64
	c                   [3]float64
	t0, life            float64
}

type paint struct {
	st stage
	fr framer
	r  rng

	blobs               []paintBlob
	seeded              bool
	nextSpray, nextIdle float64
	unit                float64
}

func (v *paint) blob(x, y, vx, vy, r float64, c [3]float64, t, life float64) {
	v.blobs = append(v.blobs, paintBlob{x: x, y: y, vx: vx, vy: vy, r: r, r0: r, c: c, t0: t, life: life})
}

func (v *paint) pick(cols [][3]float64) [3]float64 { return cols[int(v.r.next()*float64(len(cols)))] }

func (v *paint) splash(t, str float64, cols [][3]float64, x, y float64) {
	base := v.pick(cols)
	v.blob(x, y, (v.r.next()-0.5)*0.08, (v.r.next()-0.5)*0.08, 0.035+0.045*str, base, t, 6+4*v.r.next())
	arms := 3 + int(v.r.next()*4)
	for range arms {
		th := v.r.next() * 2 * math.Pi
		sp := (0.3 + 0.8*str) * (0.6 + 0.8*v.r.next())
		c := base
		if v.r.next() >= 0.65 {
			c = v.pick(cols)
		}
		seg := 3 + int(v.r.next()*4)
		for j := range seg {
			fr := float64(j+1) / float64(seg)
			rr := (0.03 - 0.019*fr) * (0.7 + 0.8*str)
			v.blob(x+math.Cos(th)*0.012*float64(j), y+math.Sin(th)*0.012*float64(j), math.Cos(th)*sp*fr, math.Sin(th)*sp*fr, rr, c, t, 5+4*v.r.next())
		}
	}
	nd := int(math.Round(5 + 14*str))
	for range nd {
		th := v.r.next() * 2 * math.Pi
		sp := (0.35 + 1.25*v.r.next()) * (0.5 + str)
		v.blob(x, y, math.Cos(th)*sp, math.Sin(th)*sp, 0.005+0.011*v.r.next(), v.pick(cols), t, 3+5*v.r.next())
	}
	if len(v.blobs) > 280 {
		v.blobs = append(v.blobs[:0], v.blobs[len(v.blobs)-280:]...)
	}
}

func (v *paint) advance(f frame, W, H float64) (dead []paintBlob) {
	t, dt, lv := f.t, f.dt, f.level
	talk := f.state == listening || f.state == responding
	cols := paintAll
	switch f.state {
	case responding:
		cols = paintHot
	case listening:
		cols = paintCool
	}
	v.unit = math.Min(H, W*1.2)
	if !v.seeded {
		v.seeded = true
		v.splash(t, 0.55, paintAll, 0, 0)
		for i := range v.blobs {
			v.blobs[i].vx *= 0.25
			v.blobs[i].vy *= 0.25
		}
	}
	jx := func() float64 { return (v.r.next() - 0.5) * 0.3 }
	jy := func() float64 { return (v.r.next() - 0.5) * 0.24 }
	if talk && f.onset {
		v.splash(t, 0.35+0.65*f.voice, cols, jx(), jy())
	}
	if talk && lv > 0.45 && t > v.nextSpray {
		v.nextSpray = t + 0.14
		v.splash(t, 0.18+0.2*lv, cols, jx(), jy())
	}
	if !talk && (t > v.nextIdle || v.nextIdle-t > 10) {
		v.nextIdle = t + 2.5 + 2*v.r.next()
		v.splash(t, 0.2, cols, jx(), jy())
	}
	kept := v.blobs[:0]
	for _, b := range v.blobs {
		age := t - b.t0
		dr := math.Exp(-2.3 * dt)
		b.vx *= dr
		b.vy *= dr
		b.x += b.vx * dt
		b.y += b.vy * dt
		if age > b.life {
			fade := (age - b.life) / 1.4
			if fade >= 1 {
				dead = append(dead, b)
				continue
			}
			b.r = b.r0 * (1 - fade)
		}
		kept = append(kept, b)
	}
	v.blobs = kept
	return dead
}

//go:embed paint.glsl
var paintShader string

func (v *paint) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(paintShader, Light|Feed|Splat); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	W, H := float64(v.st.w), float64(v.st.h)
	lv := f.level
	dead := v.advance(f, W, H)
	u := v.unit
	cx, cy := W/2, H/2

	pts := make([]Point, 0, len(v.blobs)+len(dead))
	for _, b := range v.blobs {
		br := b.r * u
		if br < 0.35 {
			continue
		}
		pts = append(pts, Point{X: float32(cx + b.x*u), Y: float32(cy + b.y*u), Size: float32(br * 4.2),
			R: float32(b.c[0] / 255), G: float32(b.c[1] / 255), B: float32(b.c[2] / 255), A: float32(br), Splat: true})
	}
	for _, b := range dead {
		pts = append(pts, Point{X: float32(cx + b.x*u), Y: float32(cy + b.y*u), Size: float32(b.r0 * u * 2.5),
			R: float32(b.c[0] / 255), G: float32(b.c[1] / 255), B: float32(b.c[2] / 255), A: 0.3, Feed: true, Disc: true, Over: true})
	}
	if err := g.Points(pts); err != nil {
		return err
	}
	vals := []float32{float32(math.Pow(0.992, f.dt*30)), float32(1 / W), float32(1 / H),
		float32(cx), float32(cy), float32(math.Max(W, H) * 0.75)}
	return g.Values(vals, float32(0.16+0.2*lv), 0.02, 1)
}
