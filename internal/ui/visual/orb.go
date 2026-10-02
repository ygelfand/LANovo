package visual

import (
	_ "embed"
	"math"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/ui"
)

// Ported from anchorapp100/ha-visualisations src/20-orb.js (MIT).

func init() { register(Orb, func() Visual { return newOrb() }) }

const (
	orbN    = 1500
	orbRing = 760
)

type ripple struct{ x, y, z, t0, amp float64 }

type orb struct {
	st stage
	fr framer

	x, y, z, nz [orbN]float64
	sx, sy, sd  [orbN]float64
	pairs       []int32
	ra, rr, rb  [orbRing]float64
	rip         []ripple
	rot         float64
	mix         float64
	r           rng
}

func newOrb() *orb {
	v := &orb{r: seeded(12)}
	ga := math.Pi * (3 - math.Sqrt(5))
	for i := range orbN {
		y := 1 - (float64(i)+0.5)/orbN*2
		rad := math.Sqrt(1 - y*y)
		th := ga * float64(i)
		v.x[i], v.y[i], v.z[i], v.nz[i] = math.Cos(th)*rad, y, math.Sin(th)*rad, v.r.next()*100
	}
	for i := range orbN {
		b1, b2, d1, d2 := -1, -1, 9.0, 9.0
		for j := range orbN {
			if j == i {
				continue
			}
			dx, dy, dz := v.x[i]-v.x[j], v.y[i]-v.y[j], v.z[i]-v.z[j]
			dd := dx*dx + dy*dy + dz*dz
			switch {
			case dd < d1:
				d2, b2, d1, b1 = d1, b1, dd, j
			case dd < d2:
				d2, b2 = dd, j
			}
		}
		if b1 > i {
			v.pairs = append(v.pairs, int32(i), int32(b1))
		}
		if b2 > i {
			v.pairs = append(v.pairs, int32(i), int32(b2))
		}
	}
	for i := range orbRing {
		v.ra[i] = v.r.next() * 2 * math.Pi
		base := 1.78
		if v.r.next() < 0.7 {
			base = 1.55
		}
		v.rr[i] = base + v.r.gauss()*0.025
		v.rb[i] = 0.3 + v.r.next()*0.7
	}
	return v
}

type orbDot struct {
	x, y, size, a float64
	c             [3]float64
}

type orbFrame struct {
	cx, cy, rs float64
	base, hot  [3]float64
	web        float64
	lines      [][4]float64
	dots       []orbDot
}

func (v *orb) frame(f frame, W, H, u float64, portrait bool) (o orbFrame) {
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
	v.rot += dt * (0.12 + 0.6*lv)
	if f.onset && talk && len(v.rip) < 5 {
		a, b := v.r.next()*2*math.Pi, math.Acos(v.r.next()*2-1)
		v.rip = append(v.rip, ripple{math.Sin(b) * math.Cos(a), math.Cos(b), math.Sin(b) * math.Sin(a), t, 0.08 + 0.16*f.voice})
	}
	live := v.rip[:0]
	for _, q := range v.rip {
		if t-q.t0 < 1.8 {
			live = append(live, q)
		}
	}
	v.rip = live
	o.base = mixc([3]float64{70, 200, 255}, [3]float64{255, 80, 190}, v.mix)
	if f.state == idle {
		o.base = mixc(o.base, [3]float64{120, 130, 255}, 0.5)
	}
	o.hot = mixc(o.base, [3]float64{255, 255, 255}, 0.65)
	o.cx, o.cy = W/2, H*0.42
	if !portrait {
		o.cy = H * 0.5
	}
	o.rs = math.Min(H, W) * 0.3 * (1 + 0.06*f.slow)
	cam, foc := 3.4, 3.4
	cr, sr := math.Cos(v.rot), math.Sin(v.rot)
	tl := 0.38 + 0.1*math.Sin(t*0.3)
	ct, stl := math.Cos(tl), math.Sin(tl)
	alphas := [6]float64{0.25, 0.4, 0.55, 0.7, 0.85, 1}
	o.dots = make([]orbDot, 0, orbN+orbRing)
	for i := range orbN {
		px, py, pz := v.x[i], v.y[i], v.z[i]
		bi := math.Abs(py) * 22
		b0 := min(int(bi), analysis.Bands-2)
		bv := f.bands[b0] + (f.bands[b0+1]-f.bands[b0])*(bi-float64(b0))
		var disp float64
		if talk {
			disp = lv * (0.06 + 0.3*bv) * (0.55 + 0.9*noise1(v.nz[i]+t*1.6))
		} else {
			disp = 0.02 * math.Sin(t*1.2+v.nz[i])
		}
		for _, q := range v.rip {
			ag := t - q.t0
			dot := math.Max(-1, math.Min(1, px*q.x+py*q.y+pz*q.z))
			ex := (math.Acos(dot) - ag*2.4) / 0.2
			disp += q.amp * math.Exp(-ex*ex) * math.Exp(-ag*1.6)
		}
		sc := 1 + disp
		X, Y, Z := px*sc, py*sc, pz*sc
		X1, Z1 := X*cr+Z*sr, -X*sr+Z*cr
		Y1, Z2 := Y*ct-Z1*stl, Y*stl+Z1*ct
		p := foc / (cam - Z2)
		v.sx[i], v.sy[i], v.sd[i] = o.cx+X1*o.rs*p, o.cy-Y1*o.rs*p, Z2
		br := (0.2+0.8*(Z2*0.5+0.5))*(0.65+0.7*lv) + disp*3
		lvl := 0
		switch {
		case br > 1.25:
			lvl = 5
		case br > 1:
			lvl = 4
		case br > 0.8:
			lvl = 3
		case br > 0.6:
			lvl = 2
		case br > 0.42:
			lvl = 1
		}
		sz := (1.3 + 1.5*(Z2*0.5+0.5)) * u * p
		o.dots = append(o.dots, orbDot{v.sx[i], v.sy[i], sz, alphas[lvl], mixc(o.base, o.hot, float64(lvl)/5)})
	}
	if lv > 0.08 {
		o.web = math.Min(0.5, 0.08+0.45*lv)
		for k := 0; k+1 < len(v.pairs); k += 2 {
			a1, a2 := v.pairs[k], v.pairs[k+1]
			if v.sd[a1] < 0.1 || v.sd[a2] < 0.1 {
				continue
			}
			o.lines = append(o.lines, [4]float64{v.sx[a1], v.sy[a1], v.sx[a2], v.sy[a2]})
		}
	}
	rt := v.rot * 1.6
	rtl := 0.3 + 0.05*math.Sin(t*0.4)
	crt, srt := math.Cos(rtl), math.Sin(rtl)
	for i := range orbRing {
		an := v.ra[i] + rt
		bb := f.bands[i%24]
		rad := v.rr[i]
		if talk {
			rad *= 1 + 0.12*bb*lv
		}
		rx, rz := math.Cos(an)*rad, math.Sin(an)*rad
		rX, rY := rx*0.94, -rx*0.34
		rY2, rZ2 := rY*crt-rz*srt, rY*srt+rz*crt
		pp := foc / (cam - rZ2*0.6)
		qx, qy, ssz := o.cx+rX*o.rs*pp, o.cy-rY2*o.rs*pp, (0.8+v.rb[i]*1.4)*u
		d := orbDot{qx + ssz/2, qy + ssz/2, ssz, 0.7, o.base}
		if v.rb[i]*(0.6+lv) > 0.7 {
			d.a, d.c = 0.95, o.hot
		}
		o.dots = append(o.dots, d)
	}
	return o
}

//go:embed orb.glsl
var orbShader string

func (v *orb) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(orbShader, Light|Lines); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	lv := f.level
	o := v.frame(f, W, H, u, Portrait(in))
	segs := make([]Segment, 0, len(o.lines))
	for _, l := range o.lines {
		segs = append(segs, Segment{X0: float32(l[0]), Y0: float32(l[1]), X1: float32(l[2]), Y1: float32(l[3]), Width: float32(math.Max(0.7*u, 1)),
			R: float32(o.base[0] / 255), G: float32(o.base[1] / 255), B: float32(o.base[2] / 255), A: float32(o.web)})
	}
	if err := g.Lines(segs); err != nil {
		return err
	}
	pts := make([]Point, 0, len(o.dots))
	for _, d := range o.dots {
		pts = append(pts, Point{X: float32(d.x), Y: float32(d.y), Size: float32(math.Max(d.size, 1)),
			R: float32(d.c[0] / 255), G: float32(d.c[1] / 255), B: float32(d.c[2] / 255), A: float32(d.a), Light: true})
	}
	if err := g.Points(pts); err != nil {
		return err
	}
	vals := []float32{float32(o.cx), float32(o.cy), float32(o.rs), float32(lv), float32(H),
		float32(o.base[0] / 255), float32(o.base[1] / 255), float32(o.base[2] / 255),
		float32(o.hot[0] / 255), float32(o.hot[1] / 255), float32(o.hot[2] / 255), float32(W)}
	return g.Values(vals, float32(0.6+0.45*lv), 0.02, 2)
}
