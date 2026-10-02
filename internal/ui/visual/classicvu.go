package visual

import (
	_ "embed"
	"hash/fnv"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/font/opentype"

	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

// Ported from anchorapp100/ha-visualisations src/11-vu.js (MIT).

func init() { register(ClassicVU, func() Visual { return &classicVU{} }) }

// EBU R68 alignment: 0 VU is -18 dBFS RMS.
const vuReference = 0.12589

const vuZero = 0.73

type vuMeter struct {
	pos, vel, peakT, lamp float64
	x, y                  float64
}

type classicVU struct {
	st stage

	w, h           int
	m              [2]vuMeter
	mw, mh, fu, tu float64
	fx, fy, fw, fh float64
	px, py, rs     float64
	clock          float64
	label          string
}

func plate(s string) string {
	return strings.Join(strings.Split(strings.TrimSpace(s), ""), "  ")
}

func (v *classicVU) key() uint64 {
	h := fnv.New32a()
	h.Write([]byte(v.label))
	return uint64(v.st.w)<<48 | uint64(v.st.h)<<32 | uint64(h.Sum32())
}

func vuTheta(a float64) float64 { return (-138 + 96*a) * math.Pi / 180 }

func vuAmp(db float64) float64 { return math.Pow(10, db/20) / 1.413 }

func (v *classicVU) layout() (title float64, labels [2][2]float64, lamp [2]float64, thinking float64) {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	v.tu = u
	lab := 34 * u
	if Portrait(ui.Rect{W: v.st.w, H: v.st.h}) {
		gap := H * 0.03
		tail := 46 * u
		v.mw = math.Min(W*0.84, (H*0.84-2*(lab+10*u)-gap-tail)/2*1.62)
		v.mh = v.mw / 1.62
		slot := v.mh + lab + 10*u
		top := (H - (2*slot + gap + tail)) / 2
		for i := range v.m {
			v.m[i].x, v.m[i].y = W/2-v.mw/2, top+float64(i)*(slot+gap)
			labels[i] = [2]float64{W / 2, v.m[i].y + v.mh + lab}
		}
		lamp = [2]float64{W / 2, labels[1][1] + 20*u}
		return top * 0.5, labels, lamp, lamp[1] + 18*u
	}
	gap := W * 0.04
	v.mw = math.Min((W*0.84-gap)/2, H*0.52*1.62)
	if short := (H*0.88 - 52*u) * 1.62; short > v.mw && v.mw < (W*0.84-gap)/2 {
		v.tu = math.Max(u, v.mw/400*0.78)
		v.mw = math.Min((W*0.84-gap)/2, (H*0.88-52*v.tu)*1.62)
		v.tu = math.Max(u, v.mw/400*0.78)
		u, lab = v.tu, 34*v.tu
	}
	v.mh = v.mw / 1.62
	my := (H - (v.mh + lab + 26*u)) / 2
	v.m[0].x, v.m[1].x = W/2-gap/2-v.mw, W/2+gap/2
	for i := range v.m {
		v.m[i].y = my
		labels[i] = [2]float64{v.m[i].x + v.mw/2, my + v.mh + lab}
	}
	lamp = [2]float64{W / 2, labels[0][1] - 4*u}
	title = my * 0.5
	if my < 20*u {
		title = -1
	}
	return title, labels, lamp, labels[0][1] + 18*u
}

func (v *classicVU) prepare() func(*image.RGBA) {
	title, labels, lamp, thinking := v.layout()
	fu := v.mw / 400
	v.fu = fu
	v.fx, v.fy, v.fw, v.fh = 12*fu, 12*fu, v.mw-24*fu, v.mh-24*fu
	v.px, v.py, v.rs = v.mw/2, v.fy+v.fh*1.08, v.fh*0.83
	for i := range v.m {
		v.m[i].peakT, v.m[i].lamp = -9, 0.6
	}
	return func(back *image.RGBA) {
		bold, medium := faces()
		v.panel(back, bold, v.tu, title, labels, lamp, thinking)
		for i := range v.m {
			v.face(back, bold, medium, v.m[i].x, v.m[i].y)
		}
	}
}

func (v *classicVU) panel(img *image.RGBA, bold *opentype.Font, u, title float64, labels [2][2]float64, lamp [2]float64, thinking float64) {
	W, H := float64(v.st.w), float64(v.st.h)
	fx.Clear(img, hex(0x121417, 1))
	r := seeded(11)
	for y := 0; y < v.st.h; y += 2 {
		a := r.next()
		c := color.NRGBA{0, 0, 0, 18}
		if a >= 0.15 {
			c = color.NRGBA{255, 255, 255, uint8((0.008 + a*0.028) * 255)}
		}
		fx.Span(img, y, 0, v.st.w, c, false)
	}
	fx.LinearRect(img, img.Bounds(), 0, 0, 0, float32(H), []fx.Stop{
		{At: 0, C: color.NRGBA{255, 255, 255, 9}}, {At: 0.45, C: color.NRGBA{255, 255, 255, 0}}, {At: 0.45, C: color.NRGBA{}}, {At: 1, C: color.NRGBA{0, 0, 0, 77}},
	}, false)
	fx.Radial(img, float32(W/2), float32(H*0.42), 0, float32(math.Max(W, H)*0.75), []fx.Stop{{At: 0, C: color.NRGBA{}}, {At: 1, C: color.NRGBA{0, 0, 0, 158}}}, false)

	for i := range v.m {
		shadow(img, v.m[i].x, v.m[i].y+8*u, v.mw, v.mh, 16*v.mw/400, 34*u, 0.85)
	}

	for i, s := range [][2]float64{{0.035, 0.06}, {0.965, 0.06}, {0.035, 0.94}, {0.965, 0.94}} {
		screw(img, s[0]*W, s[1]*H, 8*u, 0.4+float64(i)*0.9, 1.4*u, hex(0x71757b, 1), hex(0x1f2124, 1), color.NRGBA{0, 0, 0, 204})
	}

	engrave := func(txt string, x, y, size float64) {
		s := strings.Join(strings.Split(txt, ""), " ")
		letter(img, bold, size, color.NRGBA{0, 0, 0, 191}, s, x, y-1)
		letter(img, bold, size, color.NRGBA{255, 255, 255, 31}, s, x, y+1)
		letter(img, bold, size, hex(0x80868e, 1), s, x, y)
	}
	if title >= 0 {
		engrave("ASSIST · STUDIO MONITOR", W/2, title, 13*u)
	}
	engrave("MIC", labels[0][0], labels[0][1], 17*u)
	engrave("ASSIST", labels[1][0], labels[1][1], 17*u)
	engrave("THINKING", W/2, thinking, 8.5*u)
	fx.Radial(img, float32(lamp[0]), float32(lamp[1]), 0, float32(5.5*u), []fx.Stop{{At: 0, C: hex(0x4a3b16, 1)}, {At: 1, C: hex(0x1e1606, 1)}}, false)
}

func screw(img *image.RGBA, x, y, r, ang, lw float64, hi, lo, slot color.NRGBA) {
	fx.Radial(img, float32(x), float32(y), 0, float32(r), []fx.Stop{{At: 0, C: hi}, {At: 1, C: lo}}, false)
	c, s := math.Cos(ang)*r*0.7, math.Sin(ang)*r*0.7
	fx.Polyline(img, []float32{float32(x - c), float32(y - s), float32(x + c), float32(y + s)}, float32(lw), slot, false)
}

func (v *classicVU) face(img *image.RGBA, bold, medium *opentype.Font, X, Y float64) {
	fu := v.fu
	fx0, fy0, fw, fh := X+v.fx, Y+v.fy, v.fw, v.fh
	rounded(img, X, Y, v.mw, v.mh, 16*fu, func(sub *image.RGBA, ox, oy float64) {
		fx.LinearRect(img, sub.Bounds(), 0, float32(Y), 0, float32(Y+v.mh), []fx.Stop{{At: 0, C: hex(0x34373c, 1)}, {At: 1, C: hex(0x0b0c0e, 1)}}, false)
	})
	rounded(img, fx0, fy0, fw, fh, 8*fu, func(sub *image.RGBA, ox, oy float64) {
		fx.Radial(sub, float32(X+v.px-ox), float32(fy0+fh*1.15-oy), 0, float32(fw*0.95), []fx.Stop{
			{At: 0, C: hex(0xfff7e0, 1)}, {At: 0.5, C: hex(0xf7e4b3, 1)}, {At: 0.85, C: hex(0xe6c889, 1)}, {At: 1, C: hex(0xc9a45f, 1)},
		}, false)
		r := seeded(5)
		for range 900 {
			c := color.NRGBA{90, 60, 20, uint8((0.015 + r.next()*0.03) * 255)}
			fx.Square(sub, float32(fx0+r.next()*fw-ox), float32(fy0+r.next()*fh-oy), float32(fu), c, false)
		}
	})

	px, py, rs := X+v.px, Y+v.py, v.rs
	at := func(rad, a float64) (float64, float64) {
		t := vuTheta(a)
		return px + rad*math.Cos(t), py + rad*math.Sin(t)
	}
	ink, red := hex(0x1c1a16, 1), hex(0xb30d28, 1)
	arc := func(rad, a0, a1, w float64, c color.NRGBA) {
		t0, t1 := vuTheta(a0), vuTheta(a1)
		n := max(int(math.Abs(t1-t0)*rad/3), 2)
		pts := make([]float32, 0, 2*(n+1))
		for k := 0; k <= n; k++ {
			t := t0 + (t1-t0)*float64(k)/float64(n)
			pts = append(pts, float32(px+rad*math.Cos(t)), float32(py+rad*math.Sin(t)))
		}
		fx.Polyline(img, pts, float32(w), c, false)
	}
	tick := func(r0, r1, a, w float64, c color.NRGBA) {
		x0, y0 := at(r0, a)
		x1, y1 := at(r1, a)
		fx.Polyline(img, []float32{float32(x0), float32(y0), float32(x1), float32(y1)}, float32(w), c, false)
	}
	text := func(f *opentype.Font, size float64, c color.NRGBA, s string, x, y float64) {
		letter(img, f, size, c, s, x, y)
	}
	arc(rs, vuAmp(-20), vuAmp(0), 1.6*fu, ink)
	arc(rs+3*fu, vuAmp(0), 1, 6*fu, hex(0xc8102e, 1))
	for _, db := range []float64{-20, -10, -7, -5, -3, -2, -1, 0, 1, 2, 3} {
		c := ink
		if db > 0 {
			c = red
		}
		a := vuAmp(db)
		tick(rs, rs+11*fu, a, 1.7*fu, c)
		x, y := at(rs+25*fu, a)
		text(bold, 15*fu, c, strconv.Itoa(int(math.Abs(db))), x, y)
	}
	for _, db := range []float64{-15, -8.5, -6, -4, -2.5, -1.5, -0.5, 0.5, 1.5, 2.5} {
		c := ink
		if db > 0 {
			c = red
		}
		tick(rs, rs+6*fu, vuAmp(db), 1.1*fu, c)
	}
	faint := hex(0x1c1a16, 0.62)
	arc(rs-26*fu, 0.0001, 0.708, fu, faint)
	for _, pc := range []int{0, 20, 40, 60, 80, 100} {
		a := float64(pc) / 100 * 0.708
		tick(rs-26*fu, rs-31*fu, a, fu, faint)
		x, y := at(rs-40*fu, a)
		text(medium, 8.5*fu, faint, strconv.Itoa(pc), x, y)
	}
	x, y := at(rs-14*fu, 0.02)
	text(bold, 20*fu, ink, "−", x, y)
	x, y = at(rs-14*fu, 0.985)
	text(bold, 20*fu, red, "+", x, y)
	text(bold, 34*fu, hex(0x15130f, 1), "VU", px, fy0+fh*0.66)
	letterWithin(img, medium, 8*fu, hex(0x15130f, 0.5), plate(v.label), px, fy0+fh*0.77, fw*0.7)
	text(medium, 7*fu, color.NRGBA{40, 20, 0, 140}, "PEAK", fx0+fw-22*fu, fy0+34*fu)

	fx.Polyline(img, roundRectPts(fx0, fy0, fw, fh, 8*fu), float32(2*fu), color.NRGBA{0, 0, 0, 153}, false)

	lx, ly, lr := fx0+fw-22*fu, fy0+22*fu, 5*fu
	fx.Radial(img, float32(lx), float32(ly), 0, float32(lr), []fx.Stop{{At: 0, C: hex(0x6a2a2a, 1)}, {At: 1, C: hex(0x2a0909, 1)}}, false)

	cy0 := fy0 + fh*0.855
	rounded(img, fx0, fy0, fw, fh, 8*fu, func(sub *image.RGBA, ox, oy float64) {
		fx.LinearRect(img, image.Rect(int(fx0), int(cy0), int(math.Ceil(fx0+fw)), int(math.Ceil(fy0+fh))), 0, float32(cy0), 0, float32(fy0+fh), []fx.Stop{
			{At: 0, C: hex(0x1b1c1f, 1)}, {At: 1, C: hex(0x050506, 1)},
		}, false)
		fx.Span(img, int(cy0), int(fx0), int(math.Ceil(fx0+fw)), color.NRGBA{255, 255, 255, uint8(25 * math.Min(1, 1.2*fu))}, false)
	})
	screw(img, px, fy0+fh*0.93, 5*fu, -math.Atan2(0.25, 0.75), 1.3*fu, hex(0x8b8f95, 1), hex(0x2a2c2f, 1), hex(0x111111, 1))
}

func (v *classicVU) step(x Input) {
	dt := math.Min(x.Dt.Seconds(), 0.1)
	v.clock = x.Now.Seconds()
	for i, level := range []float32{x.Mic.Level, x.Speaker.Level} {
		m := &v.m[i]
		target := math.Max(0, math.Min(float64(level)/math.Sqrt2/vuReference*vuZero, 1.06))
		h := dt / 4
		for range 4 {
			m.vel += (357*(target-m.pos) - 30.6*m.vel) * h
			m.pos += m.vel * h
		}
		if m.pos < 0 {
			m.pos = 0
			m.vel = math.Max(m.vel, 0)
		}
		if m.pos > 1.07 {
			m.pos, m.vel = 1.07, -m.vel*0.25
		}
		if m.pos > vuZero {
			m.peakT = v.clock
		}
		m.lamp = follow(m.lamp, 0.55+0.45*math.Min(1, m.pos), 0.2, 0.08, dt)
	}
}

const shadeScale = 1

//go:embed classicvu.glsl
var classicVUShader string

func (v *classicVU) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(ui.Rect{W: in.W * shadeScale, H: in.H * shadeScale}) {
		return nil
	}
	if fresh {
		if err := g.Program(classicVUShader, 0); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h || v.label != x.Label {
		v.label = x.Label
		v.st.repaint(v.key(), v.prepare())
		if err := g.Texture(0, v.st.back); err != nil {
			return err
		}
		v.w, v.h = v.st.w, v.st.h
	}
	v.step(x)
	return g.Values(v.values(), 0, 0, 0)
}

func (v *classicVU) values() []float32 {
	u := make([]float32, 24)
	u[0], u[1], u[2], u[3], u[4] = shadeScale, float32(v.fu), float32(v.fw), float32(v.fh), float32(v.rs)
	for i, m := range v.m {
		peak := float32(0)
		if v.clock-m.peakT < 0.18 {
			peak = 1
		}
		b := 8 + 8*i
		u[b], u[b+1] = float32(m.x+v.fx), float32(m.y+v.fy)
		u[b+2], u[b+3] = float32(m.x+v.px), float32(m.y+v.py)
		u[b+4], u[b+5], u[b+6] = float32(vuTheta(m.pos)), float32(0.10+0.20*m.lamp), peak
	}
	return u
}

func roundRectPts(x, y, w, h, r float64) []float32 {
	var pts []float32
	corner := func(cx, cy, a0 float64) {
		for k := 0; k <= 6; k++ {
			a := a0 + float64(k)/6*math.Pi/2
			pts = append(pts, float32(cx+math.Cos(a)*r), float32(cy+math.Sin(a)*r))
		}
	}
	corner(x+w-r, y+r, -math.Pi/2)
	corner(x+w-r, y+h-r, 0)
	corner(x+r, y+h-r, math.Pi/2)
	corner(x+r, y+r, math.Pi)
	return append(pts, pts[0], pts[1])
}
