package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

// Ported from anchorapp100/ha-visualisations src/27-light-cycles.js (MIT).

func init() { register(LightCycles, func() Visual { return newLightCycles() }) }

const (
	lcHC   = 14.0
	lcZ0   = 10.6
	lcZ1   = 46.0
	lcLife = 6.0
)

var lcDirs = [4][2]float64{{0, 1}, {1, 0}, {0, -1}, {-1, 0}}

type lcPoint struct{ x, z, h, t float64 }

type cycle struct {
	x, z     float64
	dir      int
	col      [3]float64
	trail    []lcPoint
	lastTurn float64
	next     float64
	sampled  float64
	flash, h float64
}

type lightCycles struct {
	st stage
	fr framer
	r  rng

	w, h      int
	hy, cx    float64
	fp, arena float64
	cyc       [2]cycle
	pts       []float32
	quads     []Quad
	segs      []Segment
}

func newLightCycles() *lightCycles {
	v := &lightCycles{r: seeded(1982)}
	v.cyc[0] = cycle{x: -7, z: 15, dir: 0, col: [3]float64{90, 225, 255}, lastTurn: -9, sampled: -9, h: 0.4}
	v.cyc[1] = cycle{x: 7, z: 34, dir: 2, col: [3]float64{255, 150, 40}, lastTurn: -9, sampled: -9, h: 0.4}
	return v
}

func (v *lightCycles) px(x, z float64) float64 { return v.cx + v.fp*x/z }

func (v *lightCycles) py(y, z float64) float64 { return v.hy + v.fp*(lcHC-y)/z }

func (v *lightCycles) geometry() {
	W, H := float64(v.st.w), float64(v.st.h)
	v.hy, v.cx = H*0.38, W/2
	v.fp = (H - v.hy) * 10 / lcHC
	v.arena = math.Min(19, W/2/v.fp*lcZ0*0.96)
	v.w, v.h = v.st.w, v.st.h
}

func (v *lightCycles) paintBack(back *image.RGBA) {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	fx.Clear(back, color.NRGBA{0, 0, 0, 255})
	hy := v.hy
	fx.LinearRect(back, image.Rect(0, int(hy-H*0.25), v.st.w, int(hy)), 0, float32(hy-H*0.25), 0, float32(hy), []fx.Stop{
		{At: 0, C: color.NRGBA{0, 40, 70, 0}}, {At: 1, C: color.NRGBA{0, 70, 110, 89}},
	}, false)
	fx.LinearRect(back, image.Rect(0, int(hy), v.st.w, v.st.h), 0, float32(hy), 0, float32(H), []fx.Stop{
		{At: 0, C: hex(0x001426, 1)}, {At: 0.3, C: hex(0x000a14, 1)}, {At: 1, C: hex(0x000308, 1)},
	}, false)
	lw := math.Max(1, 1.2*u)
	for k := 10.0; k < 170; k += 2 {
		y := v.py(0, k)
		a := math.Max(0.03, math.Min(0.5, 0.5*math.Pow(10/k, 0.9)))
		fillRect(back, 0, y-lw/2, W, lw, nrgba([3]float64{40, 150, 255}, a))
	}
	along := func(y float64) float64 {
		t := (y - hy) / (H - hy)
		if t < 0.2 {
			return 0.14 * t / 0.2
		}
		return 0.14 + (0.5-0.14)*(t-0.2)/0.8
	}
	for i := -40; i <= 40; i++ {
		x := float64(i * 2)
		nx, ny := v.px(x, 10), v.py(0, 10)
		fxx, fy := v.px(x, 170), v.py(0, 170)
		for y := int(fy); y < min(int(ny)+1, v.st.h); y++ {
			t := (float64(y) + 0.5 - fy) / (ny - fy)
			xx := fxx + (nx-fxx)*t
			a := along(float64(y)) * math.Min(1, lw)
			x0 := int(math.Floor(xx - 0.5))
			fr := xx - 0.5 - float64(x0)
			c := [3]float64{40, 150, 255}
			fx.Span(back, y, x0, x0+1, nrgba(c, a*(1-fr)), false)
			fx.Span(back, y, x0+1, x0+2, nrgba(c, a*fr), false)
		}
	}
	A := v.arena
	corners := [][2]float64{{-A, lcZ0}, {A, lcZ0}, {A, lcZ1}, {-A, lcZ1}, {-A, lcZ0}}
	for i := 1; i < len(corners); i++ {
		a, b := corners[i-1], corners[i]
		fx.Wedge(back, float32(v.px(a[0], a[1])), float32(v.py(0, a[1])), float32(v.px(b[0], b[1])), float32(v.py(0, b[1])), float32(u), float32(u), float32(10*u), color.NRGBA{80, 200, 255, 110}, true)
	}
	v.pts = v.pts[:0]
	for _, c := range corners {
		v.pts = append(v.pts, float32(v.px(c[0], c[1])), float32(v.py(0, c[1])))
	}
	fx.Polyline(back, v.pts, float32(2*u), color.NRGBA{120, 220, 255, 217}, false)
	fillRect(back, 0, hy-0.7*u, W, math.Max(1, 1.4*u), color.NRGBA{160, 235, 255, 230})
}

func (v *lightCycles) free(c *cycle, dir int, ahead float64) bool {
	nx, nz := c.x+lcDirs[dir][0]*ahead, c.z+lcDirs[dir][1]*ahead
	return nx > -v.arena+0.5 && nx < v.arena-0.5 && nz > lcZ0+0.5 && nz < lcZ1-0.5
}

func (v *lightCycles) turn(c *cycle, t float64) {
	l, r := (c.dir+3)%4, (c.dir+1)%4
	fl, fr := v.free(c, l, 2.5), v.free(c, r, 2.5)
	switch {
	case fl && fr:
		if v.r.next() < 0.5 {
			c.dir = l
		} else {
			c.dir = r
		}
	case fl:
		c.dir = l
	case fr:
		c.dir = r
	default:
		c.dir = (c.dir + 2) % 4
	}
	c.lastTurn = t
	h := 0.4
	if len(c.trail) > 0 {
		h = c.trail[len(c.trail)-1].h
	}
	c.trail = append(c.trail, lcPoint{c.x, c.z, h, t})
}

func (v *lightCycles) advance(f frame) {
	t, dt, lv := f.t, f.dt, f.level
	for i := range v.cyc {
		c := &v.cyc[i]
		active := (f.state == listening && i == 0) || (f.state == responding && i == 1)
		spd, h := 3.4, 0.55
		if active {
			spd, h = 6+14*lv, 0.5+6*lv
		}
		if c.next == 0 || c.next-t > 10 {
			c.next = t + 1 + v.r.next()*2
		}
		if active && f.onset && t-c.lastTurn > 0.32 {
			v.turn(c, t)
			c.flash = 1
		} else if !active && t > c.next {
			v.turn(c, t)
			c.next = t + 1.2 + v.r.next()*1.6
		}
		if !v.free(c, c.dir, 0.6) {
			v.turn(c, t)
		}
		c.x += lcDirs[c.dir][0] * spd * dt
		c.z += lcDirs[c.dir][1] * spd * dt
		c.x = math.Max(-v.arena+0.3, math.Min(c.x, v.arena-0.3))
		c.z = math.Max(lcZ0+0.3, math.Min(c.z, lcZ1-0.3))
		c.h = follow(c.h, h, 0.5, 0.2, dt)
		if t-c.sampled > 1.0/30 || t < c.sampled {
			c.trail = append(c.trail, lcPoint{c.x, c.z, c.h, t})
			c.sampled = t
		}
		drop := 0
		for len(c.trail)-drop > 2 && t-c.trail[drop+1].t > lcLife {
			drop++
		}
		if drop > 0 {
			c.trail = append(c.trail[:0], c.trail[drop:]...)
		}
		c.flash = math.Max(0, c.flash-dt*4)
	}
}

//go:embed lightcycles.glsl
var lightCyclesShader string

func (v *lightCycles) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(lightCyclesShader, Light|Lines); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		v.geometry()
		back := image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
		v.paintBack(back)
		if err := g.Texture(0, back); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	t, u := f.t, v.st.u
	v.advance(f)
	v.quads, v.segs = v.quads[:0], v.segs[:0]
	vals := make([]float32, 21)
	for i := range v.cyc {
		c := &v.cyc[i]
		col, hot := c.col, mixc(c.col, [3]float64{255, 255, 255}, 0.55)
		cr, cg, cb := float32(col[0]/255), float32(col[1]/255), float32(col[2]/255)
		hr, hg, hb := float32(hot[0]/255), float32(hot[1]/255), float32(hot[2]/255)
		prev := lcPoint{}
		for j, p1 := range append(c.trail, lcPoint{c.x, c.z, c.h, t}) {
			p0 := prev
			prev = p1
			if j == 0 {
				continue
			}
			life := 1 - (t-p1.t)/lcLife
			if life <= 0 {
				continue
			}
			a := float32((math.Min(4, math.Floor(life*5)) + 0.5) / 5)
			ax, ay, bx, by := float32(v.px(p0.x, p0.z)), float32(v.py(0, p0.z)), float32(v.px(p1.x, p1.z)), float32(v.py(0, p1.z))
			ty0, ty1 := float32(v.py(p0.h, p0.z)), float32(v.py(p1.h, p1.z))
			v.quads = append(v.quads, Quad{X: [4]float32{ax, bx, bx, ax}, Y: [4]float32{ay, by, ty1, ty0}, R: cr, G: cg, B: cb, A: 0.3 * a})
			w := float32(2.2 * u)
			v.segs = append(v.segs,
				Segment{X0: ax, Y0: ty0, X1: bx, Y1: ty1, Width: w, R: hr, G: hg, B: hb, A: 0.95 * a},
				Segment{X0: ax, Y0: ay, X1: bx, Y1: by, Width: w, R: hr, G: hg, B: hb, A: 0.95 * a})
		}
		d := lcDirs[c.dir]
		zz := c.z
		sz := 10 / zz
		sx, sy := v.px(c.x, zz), v.py(0.45, zz)
		v.segs = append(v.segs, Segment{
			X0: float32(v.px(c.x-d[0]*1.3, zz-d[1]*1.3)), Y0: float32(v.py(0.45, zz-d[1]*1.3)), X1: float32(sx), Y1: float32(sy),
			Width: float32(10*u*sz + 1.5), R: hr, G: hg, B: hb, A: 1,
		})
		o := 10 * i
		vals[o], vals[o+1], vals[o+2], vals[o+3] = float32(sx), float32(v.py(0, zz)), float32(110*u*sz), float32(0.32+0.3*c.flash)
		vals[o+4], vals[o+5] = float32(sy), float32(130*u*sz*(1+0.6*c.flash)/2)
		vals[o+6], vals[o+7], vals[o+8] = cr, cg, cb
	}
	vals[20] = float32(v.st.w)
	if err := g.Quads(v.quads); err != nil {
		return err
	}
	if err := g.Lines(v.segs); err != nil {
		return err
	}
	return g.Values(vals, float32(0.7+0.5*f.level), 0.02, 2)
}
