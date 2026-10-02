package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"

	"github.com/tfriedel6/canvas"
	"github.com/tfriedel6/canvas/backend/softwarebackend"

	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

// Ported from anchorapp100/ha-visualisations src/12-aurora.js (MIT).

func init() { register(Aurora, func() Visual { return newAurora() }) }

type ribbon struct {
	own                                 int
	y, a1, k1, s1, a2, k2, s2, hr, seed float64
}

type auroraRipple struct {
	x0, t0, amp float64
	own         int
}

type star struct{ x, y, s, b, rate, ph float64 }

type shooting struct{ t0, x, y, dx, dy float64 }

type aurora struct {
	st stage
	fr framer
	r  rng

	ph        float64
	e         [2]float64
	rips      []auroraRipple
	stars     []star
	shoot     *shooting
	nextShoot float64
	ribs      []ribbon
	green     fx.Ramp
	violet    fx.Ramp
	tree      *image.RGBA
	w, h      int
}

func newAurora() *aurora {
	v := &aurora{r: seeded(21)}
	v.green = fx.NewRamp([]fx.Stop{
		{At: 0, C: color.NRGBA{200, 255, 225, 0}}, {At: 0.035, C: color.NRGBA{200, 255, 225, 230}}, {At: 0.09, C: color.NRGBA{70, 255, 155, 242}},
		{At: 0.33, C: color.NRGBA{40, 220, 165, 140}}, {At: 0.62, C: color.NRGBA{60, 160, 210, 64}}, {At: 0.85, C: color.NRGBA{120, 90, 220, 26}}, {At: 1, C: color.NRGBA{120, 90, 220, 0}},
	})
	v.violet = fx.NewRamp([]fx.Stop{
		{At: 0, C: color.NRGBA{255, 215, 245, 0}}, {At: 0.035, C: color.NRGBA{255, 215, 245, 230}}, {At: 0.09, C: color.NRGBA{255, 95, 205, 242}},
		{At: 0.33, C: color.NRGBA{190, 75, 255, 140}}, {At: 0.62, C: color.NRGBA{110, 85, 255, 64}}, {At: 0.85, C: color.NRGBA{60, 60, 200, 26}}, {At: 1, C: color.NRGBA{60, 60, 200, 0}},
	})
	v.ribs = []ribbon{
		{0, 0.52, 0.05, 0.9, 0.18, 0.03, 2.3, 0.27, 0.4, 1},
		{0, 0.6, 0.04, 1.4, -0.13, 0.025, 3.1, 0.35, 0.3, 7},
		{1, 0.45, 0.06, 0.7, 0.11, 0.035, 1.9, -0.22, 0.38, 13},
		{1, 0.56, 0.04, 1.7, -0.2, 0.02, 2.7, 0.3, 0.28, 21},
	}
	v.e = [2]float64{0.35, 0.35}
	v.nextShoot = 6 + v.r.next()*8
	return v
}

func (v *aurora) build() {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	v.stars = v.stars[:0]
	for range 240 {
		v.stars = append(v.stars, star{v.r.next() * W, math.Pow(v.r.next(), 1.4) * H * 0.78, (0.4 + v.r.next()*v.r.next()*1.6) * u, 0.25 + v.r.next()*0.75, 0.6 + v.r.next()*2.4, v.r.next() * 2 * math.Pi})
	}

	be := softwarebackend.New(v.st.w, v.st.h)
	t := canvas.New(be)
	t.SetFillStyle(color.NRGBA{1, 3, 10, 255})
	edge := func(x float64) float64 {
		return H * (0.905 + 0.035*noise1(x/W*4.3+2) - 0.02*noise1(x/W*13))
	}
	t.BeginPath()
	t.MoveTo(0, H)
	for x := 0.0; x <= W; x += math.Max(6*u, 2) {
		t.LineTo(x, edge(x))
	}
	t.LineTo(W, H)
	t.ClosePath()
	t.Fill()
	r := seeded(5)
	for range int(W / math.Max(9*u, 3)) {
		px := r.next() * W
		base := edge(px) + 2*u
		ph := H * (0.03 + r.next()*r.next()*0.085)
		pw := ph * (0.28 + r.next()*0.12)
		t.BeginPath()
		for lv := range 4 {
			ty, tw := base-ph*float64(lv)/4, pw*(1-float64(lv)/5)
			t.MoveTo(px-tw/2, ty)
			t.LineTo(px, ty-ph*0.42)
			t.LineTo(px+tw/2, ty)
			t.ClosePath()
		}
		t.Fill()
	}
	v.tree = be.Image
	v.w, v.h = v.st.w, v.st.h
}

func (v *aurora) advance(f frame) {
	t, lv := f.t, f.level
	W, H := float64(v.st.w), float64(v.st.h)
	v.ph += f.dt * (0.55 + 1.5*f.slow)
	eY, eL := 0.3+0.1*f.slow, 0.28+0.1*f.slow
	switch f.state {
	case listening:
		eY = 0.4 + 1.25*lv
	case responding:
		eL = 0.4 + 1.25*lv
	}
	v.e[0] = follow(v.e[0], eY, 0.3, 0.08, f.dt)
	v.e[1] = follow(v.e[1], eL, 0.3, 0.08, f.dt)
	if f.onset {
		own := 0
		if f.state == responding {
			own = 1
		}
		v.rips = append(v.rips, auroraRipple{0.2 + v.r.next()*0.6, t, 0.035 + 0.08*f.voice, own})
	}
	live := v.rips[:0]
	for _, q := range v.rips {
		if t-q.t0 < 3 {
			live = append(live, q)
		}
	}
	v.rips = live

	if v.shoot == nil && t > v.nextShoot {
		v.shoot = &shooting{t, W * (0.15 + v.r.next()*0.5), H * (0.05 + v.r.next()*0.25), W * (0.25 + v.r.next()*0.2), H * (0.12 + v.r.next()*0.1)}
	}
	if v.shoot != nil && (t-v.shoot.t0)/0.9 >= 1 {
		v.shoot = nil
		v.nextShoot = t + 12 + v.r.next()*14
	}
}

//go:embed aurora.glsl
var auroraShader string

func (v *aurora) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(auroraShader, Light|Pre); err != nil {
			return err
		}
	}
	if fresh || v.tree == nil || v.w != v.st.w || v.h != v.st.h {
		v.build()
		for unit, img := range map[int]*image.RGBA{0: v.base(), 1: v.starMap(), 2: v.ramps()} {
			if err := g.Texture(unit, img); err != nil {
				return err
			}
		}
	}
	v.advance(v.fr.next(x))
	return g.Values(v.values(x.Now.Seconds()), 0.9, 0.035, 2)
}

const auroraGround = 0.76

func (v *aurora) base() *image.RGBA {
	W, H := float64(v.st.w), float64(v.st.h)
	img := image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
	fx.LinearRect(img, img.Bounds(), 0, 0, 0, float32(H), []fx.Stop{
		{At: 0, C: hex(0x010208, 1)}, {At: 0.45, C: hex(0x030b1c, 1)}, {At: 0.8, C: hex(0x08193a, 1)}, {At: 1, C: hex(0x0c2144, 1)},
	}, false)
	fx.Radial(img, float32(W*0.5), float32(H*1.1), 0, float32(H*0.9), []fx.Stop{{At: 0, C: color.NRGBA{40, 90, 140, 46}}, {At: 1, C: color.NRGBA{40, 90, 140, 0}}}, false)
	ground := int(H * auroraGround)
	stars := v.starMap()
	for y := range v.st.h {
		for x := range v.st.w {
			i := img.PixOffset(x, y)
			if y < ground {
				img.Pix[i+3] = stars.Pix[i+3]
			} else {
				img.Pix[i+3] = v.tree.Pix[i+3]
			}
		}
	}
	return img
}

func (v *aurora) starMap() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
	for _, sr := range v.stars {
		s := math.Max(sr.s, 0.6)
		for y := int(sr.y); y < int(math.Ceil(sr.y+s)) && y < v.st.h; y++ {
			for x := int(sr.x); x < int(math.Ceil(sr.x+s)) && x < v.st.w; x++ {
				cx := math.Min(float64(x+1), sr.x+s) - math.Max(float64(x), sr.x)
				cy := math.Min(float64(y+1), sr.y+s) - math.Max(float64(y), sr.y)
				i := img.PixOffset(x, y)
				img.Pix[i] = uint8(sr.b * 255)
				img.Pix[i+1] = uint8(sr.rate / 3 * 255)
				img.Pix[i+2] = uint8(sr.ph / (2 * math.Pi) * 255)
				img.Pix[i+3] = uint8(clamp01(cx*cy) * 255)
			}
		}
	}
	return img
}

func (v *aurora) values(t float64) []float32 {
	u := make([]float32, 104)
	u[0], u[1], u[2] = float32(v.e[0]), float32(v.e[1]), wrap(t, 3600)
	if sh := v.shoot; sh != nil {
		sa := (t - sh.t0) / 0.9
		W, H := float64(v.st.w), float64(v.st.h)
		hx, hy := sh.x+sh.dx*sa, sh.y+sh.dy*sa
		u[3], u[4] = float32((hx-sh.dx*0.18)/W), float32((hy-sh.dy*0.18)/H)
		u[5], u[6] = float32(hx/W), float32(hy/H)
		u[7] = float32(0.9 * math.Sin(clamp01(sa)*math.Pi))
	}
	u[8] = float32(math.Min(clamp01((v.e[0]+v.e[1])*0.18), 0.5))
	if v.e[1] > v.e[0] {
		u[9] = 1
	}
	const period = 1024
	for k, rb := range v.ribs {
		b := 10 + 6*k
		u[b] = wrap(rb.s1*v.ph*3+rb.seed, 2*math.Pi)
		u[b+1] = wrap(-rb.s2*v.ph*3+rb.seed*2, 2*math.Pi)
		u[b+2] = wrap(v.ph*0.8+rb.seed, period)
		u[b+3] = wrap(v.ph*0.35+rb.seed, period)
		u[b+4] = wrap(v.ph*1.6+rb.seed*3, period)
		u[b+5] = wrap(-v.ph*0.25+rb.seed, period)
	}
	rips := v.rips[max(len(v.rips)-16, 0):]
	for i, q := range rips {
		b := 40 + 4*i
		u[b], u[b+1], u[b+2], u[b+3] = float32(q.x0), float32(t-q.t0), float32(q.amp), float32(q.own)
	}
	return u
}

func (v *aurora) ramps() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 128, 2))
	for row, r := range []*fx.Ramp{&v.green, &v.violet} {
		for k := range 128 {
			c := r[k]
			i := img.PixOffset(k, row)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
		}
	}
	return img
}
