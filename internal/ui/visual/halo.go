package visual

import (
	_ "embed"
	"math"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/ui"
)

// Ported from anchorapp100/ha-visualisations src/16-halo.js (MIT).

func init() { register(Halo, func() Visual { return &halo{r: seeded(19)} }) }

type shock struct{ t0, a float64 }

type spark struct {
	x, y, vx, vy, age, life float64
	c                       [3]float64
}

type halo struct {
	st stage
	fr framer
	r  rng

	mix    float64
	spin   float64
	length [72]float64
	shocks []shock
	sparks []spark
}

var (
	haloA = [3][3]float64{{60, 225, 255}, {80, 130, 255}, {160, 100, 255}}
	haloB = [3][3]float64{{255, 80, 190}, {255, 120, 120}, {255, 190, 110}}
)

func (v *halo) pal(p float64) [3]float64 {
	j := p * 2
	k := min(1, int(j))
	fr := j - float64(k)
	return mixc(mixc(haloA[k], haloA[k+1], fr), mixc(haloB[k], haloB[k+1], fr), v.mix)
}

func (v *halo) advance(f frame, W, H, u float64, portrait bool) (cx, cy, r0, unit float64) {
	t, dt, lv := f.t, f.dt, f.level
	talk := f.state == listening || f.state == responding
	target := v.mix
	switch f.state {
	case responding:
		target = 1
	case listening:
		target = 0
	}
	v.mix = follow(v.mix, target, 0.08, 0.08, dt)
	v.spin += dt * (0.08 + 0.5*lv)

	unit = math.Min(H, W*1.4)
	cx, cy = W/2, H*0.43
	if !portrait {
		cy = H * 0.5
	}
	r0 = unit * (0.155 + 0.02*f.slow)

	for i := range 72 {
		kk := i
		if i >= 36 {
			kk = 71 - i
		}
		bi := float64(kk) / 35 * 27
		b0 := int(bi)
		bf := bi - float64(b0)
		bv := f.bands[b0]*(1-bf) + f.bands[min(analysis.Bands-1, b0+1)]*bf
		var l float64
		if talk {
			l = 0.01 + 0.15*bv*bv*(0.5+0.8*lv)
		} else {
			l = 0.01 + 0.008*(1+math.Sin(t*1.3+float64(kk)*0.4))
		}
		v.length[i] = follow(v.length[i], l*unit, 0.55, 0.16, dt)
	}
	if f.onset && len(v.shocks) < 5 {
		v.shocks = append(v.shocks, shock{t, 0.4 + 0.6*f.voice})
	}
	live := v.shocks[:0]
	for _, q := range v.shocks {
		if t-q.t0 < 0.9 {
			live = append(live, q)
		}
	}
	v.shocks = live

	if talk && lv > 0.25 && len(v.sparks) < 160 {
		for range 3 {
			q := int(v.r.next() * 72)
			an := -math.Pi/2 + float64(q)/72*2*math.Pi + v.spin*0.15
			rad := r0 + v.length[q]
			p := q
			if q >= 36 {
				p = 72 - q
			}
			sp := (90 + 160*lv) * u
			v.sparks = append(v.sparks, spark{cx + math.Cos(an)*rad, cy + math.Sin(an)*rad, math.Cos(an) * sp, math.Sin(an) * sp, 0, 0.6 + v.r.next()*0.6, v.pal(float64(p) / 36)})
		}
	}
	kept := v.sparks[:0]
	for _, sp := range v.sparks {
		sp.age += dt
		if sp.age > sp.life {
			continue
		}
		sp.x += sp.vx * dt
		sp.y += sp.vy * dt
		kept = append(kept, sp)
	}
	v.sparks = kept
	return cx, cy, r0, unit
}

//go:embed halo.glsl
var haloShader string

func (v *halo) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(haloShader, Light|Feed|FeedHalf); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	t, lv := f.t, f.level
	cx, cy, r0, unit := v.advance(f, W, H, u, Portrait(in))

	vals := make([]float32, 94)
	longest := 0.0
	for q := range 72 {
		vals[q] = float32(v.length[q] / H)
		longest = math.Max(longest, v.length[q]/H)
	}
	vals[93] = float32(longest)
	ring := 0.0
	for i, q := range v.shocks {
		ag := (t - q.t0) / 0.9
		ring = math.Max(ring, (1-ag)*q.a)
		if i < 5 {
			vals[82+2*i], vals[83+2*i] = float32(ag), float32(q.a)
		}
	}
	vals[72], vals[73] = float32(cx/W), float32(cy/H)
	vals[74], vals[75] = float32(r0/H), float32(u/H)
	vals[76] = wrap(v.spin*0.15, 2*math.Pi)
	vals[77], vals[78], vals[79] = float32(v.mix), float32(lv), float32(ring)
	vals[80], vals[81] = float32(0.006+0.02*lv), float32(math.Pow(0.8, f.dt*30))
	vals[92] = float32(unit / H)

	pts := make([]Point, 0, len(v.sparks))
	size := float32(math.Max(4*u, 2))
	for _, sp := range v.sparks {
		a := float32(0.9 * (1 - sp.age/sp.life))
		pts = append(pts, Point{X: float32(sp.x), Y: float32(sp.y), Size: size,
			R: float32(sp.c[0] / 255), G: float32(sp.c[1] / 255), B: float32(sp.c[2] / 255), A: a, Feed: true})
	}
	if err := g.Points(pts); err != nil {
		return err
	}
	return g.Values(vals, float32(0.5+0.35*lv), 0.02, 2)
}
