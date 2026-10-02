package visual

import (
	_ "embed"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
)

// Ported from anchorapp100/ha-visualisations src/14-ribbons.js (MIT).

func init() { register(Ribbons, func() Visual { return newRibbons() }) }

type band struct {
	f, sp, ph, ofs, wid, drift float64
	b0, b1                     int
	amp, tw                    float64
}

type mote struct {
	x, y, vx, vy, life, age, s float64
	c                          [3]float64
}

type ribbons struct {
	st stage
	fr framer
	r  rng

	rb   []band
	mix  float64
	dust []mote
}

var (
	ribbonPA = [5][3]float64{{40, 210, 255}, {70, 110, 255}, {30, 255, 190}, {150, 110, 255}, {110, 225, 255}}
	ribbonPB = [5][3]float64{{255, 60, 170}, {185, 80, 255}, {255, 150, 80}, {255, 95, 215}, {130, 100, 255}}
)

func newRibbons() *ribbons {
	v := &ribbons{r: seeded(8)}
	for i := range 5 {
		v.rb = append(v.rb, band{f: 0.9 + v.r.next()*1.5, sp: 0.7 + v.r.next(), ph: v.r.next() * 2 * math.Pi, ofs: (v.r.next() - 0.5) * 0.6,
			wid: 0.3 + v.r.next()*0.22, drift: 0.12 + v.r.next()*0.2, b0: i * 5, b1: i*5 + 7, tw: v.r.next() * 2 * math.Pi})
	}
	return v
}

//go:embed ribbons.glsl
var ribbonsShader string

func (v *ribbons) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(ribbonsShader, Light|Pre); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	t, dt, lv := f.t, f.dt, f.level
	target := v.mix
	switch f.state {
	case responding:
		target = 1
	case listening:
		target = 0
	}
	v.mix = follow(v.mix, target, 0.08, 0.08, dt)
	cy := H * 0.6
	n := math.Max(40, math.Min(160, math.Round(W/(9*u))))
	vals := make([]float32, 58)
	for i := range v.rb {
		rb := &v.rb[i]
		be := 0.0
		for k := rb.b0; k < rb.b1; k++ {
			be += f.bands[k]
		}
		be /= float64(rb.b1 - rb.b0)
		rest := 0.0
		if f.state == idle {
			rest = 0.012 * (1 + math.Sin(t*0.8+float64(i)))
		}
		rb.amp = follow(rb.amp, H*(0.01+rest+0.3*lv*(0.4+0.95*be)), 0.35, 0.09, dt)
		rb.ph += dt * rb.sp * (0.9 + 4.2*f.slow)
		col := mixc(ribbonPA[i], ribbonPB[i], v.mix)
		ctr := rb.ofs + 0.3*math.Sin(t*rb.drift+float64(i)*1.3)
		b := 1 + 9*i
		vals[b], vals[b+1], vals[b+2] = float32(rb.amp/H), float32(rb.f), wrap(rb.ph, 2*math.Pi)
		vals[b+3], vals[b+4], vals[b+5] = float32(ctr), float32(rb.wid), wrap(t*1.7+rb.tw, 2*math.Pi)
		vals[b+6], vals[b+7], vals[b+8] = float32(col[0]/255), float32(col[1]/255), float32(col[2]/255)
		if (f.onset || (lv > 0.5 && v.r.next() < 0.25)) && len(v.dust) < 220 {
			count := 1
			if f.onset {
				count = 5
			}
			for range count {
				xn := math.Max(0, math.Min(1, ctr*0.5+0.5+(v.r.next()-0.5)*rb.wid*0.8))
				tt := xn*2 - 1
				pin := (1 - tt*tt) * (1 - tt*tt)
				e := math.Exp(-math.Pow((tt-ctr)/rb.wid, 2)) * pin
				top := rb.amp * e * math.Sin(rb.f*tt*math.Pi*2+rb.ph)
				v.dust = append(v.dust, mote{xn * W, cy - top*v.r.next(), (v.r.next() - 0.5) * 40 * u, -(30 + 90*v.r.next()) * u * (0.5 + lv),
					1.2 + v.r.next()*1.4, 0, (1 + v.r.next()*2) * u, col})
			}
		}
	}
	pts := make([]Point, 0, len(v.dust))
	kept := v.dust[:0]
	for _, d := range v.dust {
		d.age += dt
		if d.age > d.life {
			continue
		}
		d.x += d.vx * dt
		d.y += d.vy * dt
		d.vy *= 0.985
		c := mixc(d.c, [3]float64{255, 255, 255}, 0.5)
		pts = append(pts, Point{X: float32(d.x + d.s/2), Y: float32(d.y + d.s/2), Size: float32(math.Max(d.s, 1)),
			R: float32(c[0] / 255), G: float32(c[1] / 255), B: float32(c[2] / 255), A: float32(math.Sin(math.Pi*d.age/d.life) * 0.9), Light: true})
		kept = append(kept, d)
	}
	v.dust = kept
	if err := g.Points(pts); err != nil {
		return err
	}
	vals[50], vals[51], vals[52], vals[53] = float32(cy), float32(lv), float32(W), float32(H)
	vals[54], vals[55], vals[56] = float32(0.05*n/W), float32(u), float32(0.25+0.3*(1-math.Min(1, lv*2)))
	reach := 4 * u
	for _, rb := range v.rb {
		reach = math.Max(reach, rb.amp*1.1)
	}
	vals[57] = float32(reach + 3*u)
	return g.Values(vals, float32(0.45+0.35*lv), 0.022, 2)
}
