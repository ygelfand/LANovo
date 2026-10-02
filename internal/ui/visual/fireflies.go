package visual

import (
	_ "embed"
	"image"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
)

func init() { register(Fireflies, func() Visual { return &fireflies{r: seeded(1820)} }) }

const (
	fireflyCount = 140
	fireflyFlash = 0.14
)

type firefly struct {
	x, y, z, vx, vy float64
	phase, period   float64
}

type fireflies struct {
	st stage
	fr framer
	r  rng

	w, h  int
	flies []firefly
	lift  float64
	gain  float64
	pts   []Point
}

func (v *fireflies) horizon() float64 { return float64(v.st.h) * 0.56 }

func (v *fireflies) treeline(x float64) float64 {
	W := float64(v.st.w)
	k := x / W * 2 * math.Pi
	top := v.horizon() - float64(v.st.h)*(0.03+0.02*math.Sin(k*1.3+0.7)+0.012*math.Sin(k*3.1+2.1))
	u := v.st.u
	cell := int(x / (14 * u))
	c := (float64(cell) + 0.5) * 14 * u
	dx := math.Abs(x-c) / (14 * u)
	return top - math.Max(0, 1-dx*1.3)*(8+30*cellHash(cell, 3, 21))*u
}

func (v *fireflies) paint() *image.RGBA {
	W, H := v.st.w, v.st.h
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	hz := v.horizon()
	u := v.st.u
	sky, below := make([][3]uint8, H), make([][3]uint8, H)
	for y := range H {
		fy := float64(y) + 0.5
		t := fy / hz
		r, g, b := 0.008+0.03*t*t, 0.012+0.05*t*t, 0.035+0.06*t*t
		r += 0.035 * math.Pow(t, 6)
		g += 0.02 * math.Pow(t, 6)
		sky[y] = [3]uint8{uint8(255 * r), uint8(255 * g), uint8(255 * b)}
		if fy < hz+float64(H)*0.01 {
			r, g, b = 0.006, 0.012, 0.014
		} else {
			t := (fy - hz) / (float64(H) - hz)
			r, g, b = 0.02*(1-t)+0.01, 0.045*(1-t)+0.02, 0.04*(1-t)+0.018
		}
		below[y] = [3]uint8{uint8(255 * r), uint8(255 * g), uint8(255 * b)}
	}
	for x := range W {
		top := v.treeline(float64(x) + 0.5)
		for y := range H {
			c := below[y]
			if float64(y)+0.5 < top {
				c = sky[y]
			}
			o := img.PixOffset(x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = c[0], c[1], c[2], 255
		}
	}
	cs := int(math.Max(8, 26*u))
	for cy := 0; cy*cs < int(hz); cy++ {
		for cx := 0; cx*cs < W; cx++ {
			if cellHash(cx, cy, 5) < 0.62 {
				continue
			}
			x := cx*cs + int(cellHash(cx, cy, 6)*float64(cs))
			y := cy*cs + int(cellHash(cx, cy, 7)*float64(cs))
			if x >= W || float64(y) >= v.treeline(float64(x))-2 {
				continue
			}
			k := 0.25 + 0.6*cellHash(cx, cy, 8)*(1-float64(y)/hz)
			o := img.PixOffset(x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2] = uint8(255*k), uint8(255*k), uint8(250*k)
		}
	}
	blades := int(float64(W) / (3 * u))
	for i := range blades {
		bx := float64(W) * cellHash(i, 0, 31)
		depth := cellHash(i, 1, 31)
		base := float64(H) * (1.02 - 0.08*depth)
		tall := float64(H) * (0.1 + 0.18*cellHash(i, 2, 31)) * (1 - 0.5*depth)
		lean := (cellHash(i, 3, 31) - 0.5) * tall * 0.7
		wide := (2 + 3*(1-depth)) * u
		shade := 0.002 + 0.008*depth
		steps := int(tall / 1.5)
		for s := range steps {
			t := float64(s) / float64(steps)
			px := bx + lean*t*t
			py := base - tall*t
			rw := wide * (1 - t)
			for dx := -int(rw) - 1; dx <= int(rw)+1; dx++ {
				xx, yy := int(px)+dx, int(py)
				if xx < 0 || xx >= W || yy < 0 || yy >= H || math.Abs(float64(dx)) > rw+0.5 {
					continue
				}
				o := img.PixOffset(xx, yy)
				img.Pix[o], img.Pix[o+1], img.Pix[o+2] = uint8(255*shade*0.6), uint8(255*shade*1.2), uint8(255*shade)
			}
		}
	}
	return img
}

func (v *fireflies) spawn() {
	v.flies = v.flies[:0]
	for range fireflyCount {
		v.flies = append(v.flies, v.fly())
	}
}

func (v *fireflies) fly() firefly {
	W, H := float64(v.st.w), float64(v.st.h)
	return firefly{
		x: W * v.r.next(), y: H * (0.55 + 0.4*v.r.next()), z: v.r.next(),
		phase: v.r.next(), period: 2.5 + 3.5*v.r.next(),
	}
}

func (v *fireflies) advance(f frame) {
	dt := f.dt
	W, H := float64(v.st.w), float64(v.st.h)
	talk := f.state == listening || f.state == responding
	lift, gain := 0.0, 0.8
	if talk {
		lift, gain = f.level, 0.9+0.6*f.level
	}
	v.lift = follow(v.lift, lift, 0.2, 0.05, dt)
	v.gain = follow(v.gain, gain, 0.5, 0.1, dt)
	for i := range v.flies {
		fl := &v.flies[i]
		scale := H * (0.004 + 0.01*fl.z)
		fl.vx += (v.r.next() - 0.5) * scale * 6 * dt
		fl.vy += (v.r.next() - 0.5) * scale * 6 * dt
		fl.vx *= math.Pow(0.3, dt)
		fl.vy *= math.Pow(0.3, dt)
		rise := 0.0
		if fl.phase < fireflyFlash {
			t := fl.phase / fireflyFlash
			rise = scale * (8*t - 2) * (1 + v.lift)
		}
		fl.x += fl.vx * dt
		fl.y += (fl.vy - rise) * dt
		low, high := H*(0.97-0.12*(1-fl.z)), H*(0.52-0.25*v.lift)
		if fl.y > low {
			fl.vy -= scale * 2 * dt
		}
		if fl.y < high {
			fl.vy += scale * 2 * dt
		}
		fl.x = math.Max(-W*0.02, math.Min(fl.x, W*1.02))
		was := fl.phase
		fl.phase += dt / fl.period
		if f.onset && talk && fl.phase > 0.55 && v.r.next() < 0.5+0.4*f.level {
			fl.phase = 0
		}
		fl.phase -= math.Floor(fl.phase)
		if was < fireflyFlash && fl.phase >= fireflyFlash {
			fl.x += (v.r.next() - 0.5) * W * 0.06
			fl.y += (v.r.next() - 0.3) * H * 0.04
		}
	}
}

func (fl *firefly) bright() float64 {
	if fl.phase >= fireflyFlash {
		return 0
	}
	t := fl.phase / fireflyFlash
	if t < 0.2 {
		return smoothstep(0, 0.2, t)
	}
	return math.Pow(1-(t-0.2)/0.8, 1.6)
}

//go:embed fireflies.glsl
var firefliesShader string

func (v *fireflies) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(firefliesShader, Light); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		if err := g.Texture(0, v.paint()); err != nil {
			return err
		}
		v.w, v.h = v.st.w, v.st.h
		v.spawn()
	}
	f := v.fr.next(x)
	v.advance(f)
	u := v.st.u
	v.pts = v.pts[:0]
	for i := range v.flies {
		fl := &v.flies[i]
		b := fl.bright() * v.gain
		if b <= 0.01 {
			continue
		}
		near := 0.5 + 1.1*fl.z
		v.pts = append(v.pts,
			Point{X: float32(fl.x), Y: float32(fl.y), Size: float32(56 * u * near), R: 0.55, G: 0.85, B: 0.2, A: float32(0.18 * b), Round: true, Light: true},
			Point{X: float32(fl.x), Y: float32(fl.y), Size: float32(16 * u * near), R: 0.8, G: 1, B: 0.35, A: float32(0.6 * b), Round: true, Light: true},
			Point{X: float32(fl.x), Y: float32(fl.y), Size: float32(6 * u * near), R: 1, G: 1, B: 0.7, A: float32(math.Min(1, b)), Round: true})
	}
	if err := g.Points(v.pts); err != nil {
		return err
	}
	ph := func(k float64) float32 { return float32(math.Mod(f.t*k, 1)) }
	vals := []float32{float32(v.st.w), float32(v.st.h), ph(0.013), ph(0.009), float32(v.gain), float32(v.horizon() / float64(v.st.h))}
	return g.Values(vals, 0.55, 0.03, 2)
}
