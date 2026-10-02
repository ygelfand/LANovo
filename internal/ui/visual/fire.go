package visual

import (
	_ "embed"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
)

func init() { register(Fire, func() Visual { return &fire{r: seeded(1666)} }) }

const fireSparks = 220

type ember struct{ x, y, vx, vy, life, age float64 }

type fire struct {
	st stage
	fr framer
	r  rng

	heat, kick float64
	sparks     []ember
	pts        []Point
}

func (v *fire) advance(f frame) {
	dt := f.dt
	W, H := float64(v.st.w), float64(v.st.h)
	talk := f.state == listening || f.state == responding
	want := 0.45
	if talk {
		want = 0.5 + 0.75*f.level
	}
	v.heat = follow(v.heat, want, 0.6, 0.2, dt)
	if f.onset && talk {
		v.kick = 1
	}
	v.kick = math.Max(0, v.kick-dt*2.5)
	n := int((4+40*f.level+60*v.kick)*dt + v.r.next())
	for range n {
		if len(v.sparks) >= fireSparks {
			break
		}
		v.sparks = append(v.sparks, ember{
			x: W * (0.05 + 0.9*v.r.next()), y: H * (0.9 + 0.08*v.r.next()),
			vx: (v.r.next() - 0.5) * W * 0.08, vy: -H * (0.25 + 0.35*v.r.next()) * (0.8 + 0.6*v.heat),
			life: 1 + 1.6*v.r.next(),
		})
	}
	kept := v.sparks[:0]
	for _, s := range v.sparks {
		s.age += dt
		if s.age > s.life || s.y < 0 {
			continue
		}
		s.vx += (v.r.next() - 0.5) * W * 0.35 * dt
		s.vy *= math.Pow(0.7, dt)
		s.x += s.vx * dt
		s.y += s.vy * dt
		kept = append(kept, s)
	}
	v.sparks = kept
}

//go:embed fire.glsl
var fireShader string

func (v *fire) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(fireShader, Light|Feed|FeedHalf|FeedFloat); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	v.advance(f)
	u := v.st.u
	v.pts = v.pts[:0]
	for _, s := range v.sparks {
		a := float32(1 - s.age/s.life)
		v.pts = append(v.pts, Point{X: float32(s.x), Y: float32(s.y), Size: float32(math.Max(3, 4.5*u)), R: 1, G: 0.7 * a, B: 0.3 * a, A: a, Round: true, Light: true})
	}
	if err := g.Points(v.pts); err != nil {
		return err
	}
	W, H := float64(v.st.w), float64(v.st.h)
	vals := []float32{float32(W), float32(H), float32(math.Mod(f.t, 1000)), float32(v.heat + 0.35*v.kick), float32(u)}
	return g.Values(vals, float32(0.5+0.4*v.heat), 0.02, 2)
}
