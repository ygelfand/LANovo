package visual

import (
	_ "embed"
	"image"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
)

func init() { register(Thunderstorm, func() Visual { return &thunder{r: seeded(1752)} }) }

const (
	boltSteps  = 64
	stormNoise = 256
)

type bolt struct {
	x, y       [][]float64
	age, life  float64
	width      float64
	pulses     int
	fx, fy, in float64
}

type thunder struct {
	st stage
	fr framer
	r  rng

	w, h     int
	bolts    []bolt
	flash    float64
	fx, fy   float64
	rain     float64
	wind     float64
	next     float64
	cooldown float64
	pts      []Point
}

func (v *thunder) ground(x float64) float64 {
	W, H := float64(v.st.w), float64(v.st.h)
	k := x / W * 2 * math.Pi
	return H * (0.8 - 0.035*math.Sin(k*0.9+1.3) - 0.02*math.Sin(k*2.3+0.4) - 0.008*math.Sin(k*5.1))
}

func (v *thunder) paint() *image.RGBA {
	w, h := max(v.st.w/2, 1), max(v.st.h/2, 1)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	u := v.st.u
	for x := range w {
		fx := (float64(x) + 0.5) * 2
		top := v.ground(fx)
		cell := int(fx / (9 * u))
		tree := cellHash(cell, 0, 7)
		if tree > 0.35 {
			c := (float64(cell) + 0.5) * 9 * u
			dx := math.Abs(fx-c) / (9 * u)
			top -= math.Max(0, 1-dx*1.6) * (10 + 26*tree) * u
		}
		for y := range h {
			fy := (float64(y) + 0.5) * 2
			if fy < top {
				continue
			}
			depth := clamp01((fy - top) / (float64(v.st.h) * 0.2))
			r, g, b := 0.012+0.01*(1-depth), 0.016+0.012*(1-depth), 0.026+0.016*(1-depth)
			o := img.PixOffset(x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = uint8(255*r), uint8(255*g), uint8(255*b), 255
		}
	}
	for i := range 14 {
		fx := float64(v.st.w) * (0.05 + 0.9*cellHash(i, 1, 9))
		fy := v.ground(fx) + (4+10*cellHash(i, 2, 9))*u
		x, y := int(fx/2), int(fy/2)
		if x >= 0 && x < w && y >= 0 && y < h {
			o := img.PixOffset(x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2] = 200, 140, 60
		}
	}
	return img
}

func (v *thunder) noise() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, stormNoise, stormNoise))
	fbm := func(x, y int, salt uint32) float64 {
		s, a, tot := 0.0, 0.5, 0.0
		for _, period := range []int{4, 8, 16, 32, 64} {
			cs := stormNoise / period
			fx, fy := float64(x)/float64(cs), float64(y)/float64(cs)
			ix, iy := int(fx), int(fy)
			tx, ty := smoothstep(0, 1, fx-float64(ix)), smoothstep(0, 1, fy-float64(iy))
			at := func(i, j int) float64 { return cellHash(i%period, j%period, salt+uint32(period)) }
			n := (at(ix, iy)*(1-tx)+at(ix+1, iy)*tx)*(1-ty) + (at(ix, iy+1)*(1-tx)+at(ix+1, iy+1)*tx)*ty
			s += n * a
			tot += a
			a *= 0.55
		}
		return s / tot
	}
	for y := range stormNoise {
		for x := range stormNoise {
			o := img.PixOffset(x, y)
			img.Pix[o] = uint8(255 * fbm(x, y, 11))
			img.Pix[o+1] = uint8(255 * fbm(x, y, 29))
			img.Pix[o+2] = uint8(255 * cellHash(x, y, 43))
			img.Pix[o+3] = 255
		}
	}
	return img
}

func (v *thunder) path(x0, y0, x1, y1, rough float64) ([]float64, []float64) {
	px, py := make([]float64, boltSteps+1), make([]float64, boltSteps+1)
	px[0], py[0], px[boltSteps], py[boltSteps] = x0, y0, x1, y1
	for step := boltSteps; step > 1; step >>= 1 {
		for a := 0; a < boltSteps; a += step {
			b, m := a+step, a+step/2
			dx, dy := px[b]-px[a], py[b]-py[a]
			j := (v.r.next() - 0.5) * 2 * rough
			px[m] = (px[a]+px[b])/2 - dy*j
			py[m] = (py[a]+py[b])/2 + dx*j
		}
		rough *= 0.6
	}
	return px, py
}

func (v *thunder) strike(ground bool) {
	W, H := float64(v.st.w), float64(v.st.h)
	x0 := W * (0.1 + 0.8*v.r.next())
	y0 := H * (0.12 + 0.12*v.r.next())
	b := bolt{life: 0.45 + 0.35*v.r.next(), pulses: 2 + int(v.r.next()*3), width: 0.8 + 0.6*v.r.next()}
	if !ground {
		b.fx, b.fy, b.in = x0, y0, 0.6+0.4*v.r.next()
		v.bolts = append(v.bolts, b)
		return
	}
	x1 := x0 + (v.r.next()-0.5)*W*0.25
	px, py := v.path(x0, y0, x1, v.ground(x1), 0.28)
	b.x, b.y = append(b.x, px), append(b.y, py)
	for range 1 + int(v.r.next()*3) {
		k := 8 + int(v.r.next()*float64(boltSteps/2))
		sx, sy := px[k], py[k]
		l := (H - sy) * (0.15 + 0.25*v.r.next())
		ang := (v.r.next() - 0.5) * 1.6
		bx, by := v.path(sx, sy, sx+math.Sin(ang)*l, sy+math.Cos(ang)*l, 0.3)
		b.x, b.y = append(b.x, bx), append(b.y, by)
	}
	b.fx, b.fy, b.in = x0, y0, 1
	v.bolts = append(v.bolts, b)
}

func (b *bolt) glow() float64 {
	t := b.age / b.life
	if t >= 1 {
		return 0
	}
	p := t * float64(b.pulses)
	return (0.35 + 0.65*math.Pow(1-(p-math.Floor(p)), 3)) * (1 - t) * b.in
}

func (v *thunder) advance(f frame) {
	dt := f.dt
	talk := f.state == listening || f.state == responding
	want := 0.45
	if talk {
		want = 0.55 + 0.45*f.level
	}
	v.rain = follow(v.rain, want, 0.2, 0.05, dt)
	v.wind = 0.12 + 0.08*math.Sin(f.t*0.13)
	v.cooldown -= dt
	if f.onset && talk && v.cooldown <= 0 {
		v.strike(v.r.next() < 0.3+0.6*f.level)
		v.cooldown = 0.5
	}
	v.next -= dt
	if v.next <= 0 {
		v.strike(v.r.next() < 0.35)
		v.next = 4 + 9*v.r.next()
	}
	v.flash = 0
	kept := v.bolts[:0]
	for _, b := range v.bolts {
		b.age += dt
		if b.age >= b.life {
			continue
		}
		if g := b.glow(); g > v.flash {
			v.flash, v.fx, v.fy = g, b.fx, b.fy
		}
		kept = append(kept, b)
	}
	v.bolts = kept
}

//go:embed thunder.glsl
var thunderShader string

func (v *thunder) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(thunderShader, Light); err != nil {
			return err
		}
		if err := g.Texture(1, v.noise()); err != nil {
			return err
		}
		v.next = 1 + 2*v.r.next()
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		if err := g.Texture(0, v.paint()); err != nil {
			return err
		}
		v.w, v.h = v.st.w, v.st.h
	}
	f := v.fr.next(x)
	v.advance(f)
	u := v.st.u
	v.pts = v.pts[:0]
	for _, b := range v.bolts {
		gl := b.glow()
		for n := range b.x {
			wk := b.width
			if n > 0 {
				wk *= 0.55
			}
			for _, layer := range []struct{ w, a, r, g, b float64 }{{18, 0.05, 0.45, 0.5, 1}, {6, 0.22, 0.7, 0.75, 1}, {3, 0.6, 1, 1, 1}} {
				a := float32(math.Min(layer.a*gl*1.4, 1))
				size := math.Max(2, layer.w*u*wk)
				step := size * 0.35
				px, py := b.x[n], b.y[n]
				for i := range boltSteps {
					dx, dy := px[i+1]-px[i], py[i+1]-py[i]
					k := max(1, int(math.Hypot(dx, dy)/step))
					for j := range k {
						t := float64(j) / float64(k)
						v.pts = append(v.pts, Point{X: float32(px[i] + dx*t), Y: float32(py[i] + dy*t), Size: float32(size),
							R: float32(layer.r), G: float32(layer.g), B: float32(layer.b), A: a, Round: true, Light: true})
					}
				}
			}
		}
	}
	if err := g.Points(v.pts); err != nil {
		return err
	}
	W, H := float64(v.st.w), float64(v.st.h)
	ph := func(k float64) float32 { return float32(math.Mod(f.t*k, 1)) }
	vals := []float32{float32(W), float32(H), ph(0.05), float32(v.flash), float32(v.fx / H), float32(v.fy / H), float32(v.rain), float32(v.wind),
		ph(0.006), ph(0.011), ph(0.002), ph(0.02), ph(0.004), ph(0.12), ph(0.2)}
	return g.Values(vals, float32(0.3+0.5*v.flash), 0.03, 2)
}
