package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

// Ported from anchorapp100/ha-visualisations src/15-galaxy.js (MIT).

func init() { register(Galaxy, func() Visual { return newGalaxy() }) }

const galaxyN = 2600

type galaxyRing struct{ t0, a float64 }

type galaxy struct {
	st stage
	fr framer

	pr, pt, pz, ps, pb [galaxyN]float64
	pc                 [galaxyN]uint8
	rot, mix           float64
	rings              []galaxyRing
}

var (
	galaxyPA = [5][3]float64{{255, 236, 210}, {160, 196, 255}, {110, 225, 255}, {255, 120, 200}, {150, 130, 255}}
	galaxyPB = [5][3]float64{{255, 225, 235}, {255, 150, 210}, {220, 120, 255}, {255, 90, 160}, {255, 170, 120}}
)

const (
	galaxyTilt = 1.08
	galaxyRoll = -0.38
)

func newGalaxy() *galaxy {
	v := &galaxy{}
	r := seeded(77)
	for i := range galaxyN {
		k := r.next()
		b, sz := 0.35+r.next()*0.65, 0.8+r.next()*r.next()*1.8
		var rr, th, z float64
		var c uint8
		switch {
		case k < 0.17:
			rr, th, z, c, b = math.Abs(r.gauss())*0.09, r.next()*2*math.Pi, r.gauss()*0.05, 0, 0.6+r.next()*0.4
		case k < 0.8:
			rr = math.Min(1.05, 0.07-math.Log(1-r.next()*0.97)*0.27)
			arm := 0.0
			if r.next() >= 0.5 {
				arm = math.Pi
			}
			th = arm + 2.75*math.Log(rr/0.07) + r.gauss()*(0.1+0.08*(1-rr))
			z = r.gauss() * 0.015
			switch p := r.next(); {
			case p < 0.12:
				c = 3
			case p < 0.55:
				c = 1
			case p < 0.8:
				c = 2
			default:
				c = 4
			}
			b, sz = 0.55+r.next()*0.45, sz*1.15
		default:
			rr, th, z = 0.1+r.next()*0.95, r.next()*2*math.Pi, r.gauss()*0.05
			c = 4
			if r.next() < 0.6 {
				c = 1
			}
			b, sz = 0.2+r.next()*0.35, sz*0.8
		}
		v.pr[i], v.pt[i], v.pz[i], v.ps[i], v.pc[i], v.pb[i] = rr, th, z, sz, c, b
	}
	return v
}

var galaxyAlpha = [3]float64{0.35, 0.7, 1}

func (v *galaxy) advance(f frame) {
	t, dt, lv := f.t, f.dt, f.level
	target := v.mix
	switch f.state {
	case responding:
		target = 1
	case listening:
		target = 0
	}
	v.mix = follow(v.mix, target, 0.06, 0.06, dt)
	v.rot += dt * (0.05 + 0.75*lv)
	if f.onset && len(v.rings) < 3 {
		v.rings = append(v.rings, galaxyRing{t, 0.05 + 0.08*f.voice})
	}
	live := v.rings[:0]
	for _, q := range v.rings {
		if t-q.t0 < 1.6 {
			live = append(live, q)
		}
	}
	v.rings = live
}

func (v *galaxy) frame(f frame, W, H float64) (cx, cy, rg float64, cols [5][3]float64) {
	rg = math.Min(W*0.46, H*0.58) * (1 + 0.05*f.slow)
	for k := range cols {
		cols[k] = mixc(galaxyPA[k], galaxyPB[k], v.mix)
	}
	return W / 2, H / 2, rg, cols
}

func (v *galaxy) stars(f frame, cx, cy, rg, u float64, emit func(x, y, sz float64, c uint8, lvl int)) {
	t, lv := f.t, f.level
	cT, sT, cR, sR := math.Cos(galaxyTilt), math.Sin(galaxyTilt), math.Cos(galaxyRoll), math.Sin(galaxyRoll)
	tw := 0.06*math.Sin(t*0.6) + 0.05*lv
	for i := range galaxyN {
		rr := v.pr[i]
		dr, br := 0.0, 0.0
		for _, q := range v.rings {
			age := t - q.t0
			ex := (rr - age*0.62) / 0.07
			w := math.Exp(-ex*ex) * math.Exp(-age*1.4)
			dr += q.a * w
			br += w
		}
		r1 := rr + dr
		th := v.pt[i] + v.rot + tw/(0.3+rr)
		px, py, z := r1*math.Cos(th), r1*math.Sin(th), v.pz[i]
		yy := py*cT - z*sT
		sx, sy := px*cR-yy*sR, px*sR+yy*cR
		bb := v.pb[i]*(0.7+0.6*lv) + br*1.2
		lvl := 0
		switch {
		case bb > 0.95:
			lvl = 2
		case bb > 0.55:
			lvl = 1
		}
		emit(cx+sx*rg, cy+sy*rg, v.ps[i]*u*(1+0.5*br), v.pc[i], lvl)
	}
}

func (v *galaxy) backdrop(back *image.RGBA) {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	fx.Clear(back, color.NRGBA{1, 2, 7, 255})
	for _, n := range []struct {
		x, y float64
		c    [3]float64
	}{{0.3, 0.3, [3]float64{40, 60, 140}}, {0.8, 0.75, [3]float64{110, 40, 120}}, {0.55, 0.15, [3]float64{30, 90, 130}}} {
		fx.Radial(back, float32(n.x*W), float32(n.y*H), 0, float32(H*0.6), []fx.Stop{{At: 0, C: nrgba(n.c, 0.16)}, {At: 1, C: nrgba(n.c, 0)}}, false)
	}
	r := seeded(4)
	for range 420 {
		b := 0.2 + r.next()*r.next()*0.8
		sz := (0.5 + r.next()*1.1) * u
		fx.Square(back, float32(r.next()*W), float32(r.next()*H), float32(math.Max(sz, 0.6)), nrgba([3]float64{220, 230, 255}, b), false)
	}
}

//go:embed galaxy.glsl
var galaxyShader string

func (v *galaxy) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	resized := v.st.w != in.W || v.st.h != in.H
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(galaxyShader, Light|Feed); err != nil {
			return err
		}
	}
	if fresh || resized {
		back := image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
		v.backdrop(back)
		if err := g.Texture(0, back); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	t, lv := f.t, f.level
	v.advance(f)
	cx, cy, rg, cols := v.frame(f, W, H)

	pts := make([]Point, 0, galaxyN)
	v.stars(f, cx, cy, rg, u, func(x, y, sz float64, c uint8, lvl int) {
		k := cols[c]
		pts = append(pts, Point{X: float32(x + sz/2), Y: float32(y + sz/2), Size: float32(math.Max(sz, 1)),
			R: float32(k[0] / 255), G: float32(k[1] / 255), B: float32(k[2] / 255), A: float32(galaxyAlpha[lvl]), Feed: true})
	})
	if err := g.Points(pts); err != nil {
		return err
	}

	vals := make([]float32, 22)
	vals[0] = float32(math.Pow(0.72, f.dt*30))
	vals[1], vals[2], vals[3] = float32(cx/W), float32(cy/H), float32(rg/H)
	vals[4], vals[5], vals[6] = float32(math.Cos(galaxyTilt)), float32(galaxyRoll), float32(u/H)
	for i, q := range v.rings {
		ag := t - q.t0
		if rd := ag * 0.62 * rg; rd >= 2 && i < 3 {
			vals[8+2*i], vals[9+2*i] = float32(rd/H), float32(math.Exp(-ag*2.6)*0.32)
		}
	}
	vals[14], vals[15] = float32(rg*(0.13+0.08*lv)/H), float32(lv)
	for k := range 3 {
		vals[16+k] = float32(cols[0][k] / 255)
		vals[19+k] = float32(cols[2][k] / 255)
	}
	return g.Values(vals, float32(0.75+0.6*lv), 0.03, 2)
}
