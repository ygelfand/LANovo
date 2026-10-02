package visual

import (
	_ "embed"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
)

func init() { register(InkInWater, func() Visual { return &ink{r: seeded(1450)} }) }

const inkDrops = 12

var inkColors = [][3]float32{
	{0.95, 0.12, 0.5},
	{0.1, 0.65, 0.95},
	{1.0, 0.55, 0.1},
	{0.55, 0.2, 1.0},
	{0.95, 0.15, 0.12},
	{0.15, 0.9, 0.6},
}

type inkDrop struct {
	x, y, vx, vy, age, life, size float64
	c                             [3]float32
}

type ink struct {
	st stage
	fr framer
	r  rng

	drops    []inkDrop
	stir     float64
	next     float64
	cooldown float64
	pts      []Point
}

func (v *ink) drop(strength float64) {
	if len(v.drops) >= inkDrops {
		return
	}
	W, H := float64(v.st.w), float64(v.st.h)
	v.drops = append(v.drops, inkDrop{
		x: W * (0.12 + 0.76*v.r.next()), y: -H * 0.02,
		vx: (v.r.next() - 0.5) * W * 0.05, vy: H * (0.35 + 0.35*v.r.next()) * (0.7 + 0.5*strength),
		life: 2.2 + 1.5*v.r.next(), size: (0.8 + 0.5*v.r.next()) * (0.7 + 0.5*strength),
		c: inkColors[int(v.r.next()*float64(len(inkColors)))%len(inkColors)],
	})
}

func (v *ink) advance(f frame) {
	dt := f.dt
	talk := f.state == listening || f.state == responding
	want := 0.7
	if talk {
		want = 0.9 + 2.2*f.level
	}
	v.stir = follow(v.stir, want, 0.5, 0.12, dt)
	v.cooldown -= dt
	if f.onset && talk && v.cooldown <= 0 {
		v.drop(0.5 + f.level)
		v.cooldown = 0.3
	}
	v.next -= dt
	if v.next <= 0 {
		v.drop(0.5)
		v.next = 2.5 + 3*v.r.next()
		if talk {
			v.next *= 0.5
		}
	}
	kept := v.drops[:0]
	for _, d := range v.drops {
		d.age += dt
		if d.age > d.life {
			continue
		}
		d.vy *= math.Pow(0.3, dt)
		d.vx *= math.Pow(0.4, dt)
		d.x += d.vx * dt
		d.y += d.vy * dt
		kept = append(kept, d)
	}
	v.drops = kept
}

//go:embed ink.glsl
var inkShader string

func (v *ink) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(inkShader, Light|Feed|FeedHalf|FeedFloat); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	v.advance(f)
	u := v.st.u
	v.pts = v.pts[:0]
	for _, d := range v.drops {
		k := 1 - d.age/d.life
		a := float32(0.3 * k * smoothstep(0, 0.35, d.age))
		for i := range 3 {
			jx, jy := (v.r.next()-0.5)*14*u, (v.r.next()-0.5)*14*u-float64(i)*6*u
			v.pts = append(v.pts, Point{X: float32(d.x + jx), Y: float32(d.y + jy), Size: float32((34 + 40*k) * d.size * u),
				R: d.c[0], G: d.c[1], B: d.c[2], A: a, Feed: true, Round: true})
		}
	}
	if err := g.Points(v.pts); err != nil {
		return err
	}
	vals := []float32{float32(v.st.w), float32(v.st.h), float32(math.Mod(f.t, 1000)), float32(v.stir), 0.997}
	return g.Values(vals, 0.45, 0.03, 2)
}
