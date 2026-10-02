package visual

import (
	_ "embed"
	"image"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/glow"
)

func init() { register(RainOnGlass, func() Visual { return &rain{r: seeded(1066)} }) }

const rainMax = 2400

type raindrop struct {
	x, y, r, vy, phase float64
	run, dry           bool
	trail              float64
}

type rain struct {
	st stage
	fr framer
	r  rng

	w, h  int
	drops []raindrop
	spawn float64
	fog   float64
	pts   []Point
}

func (v *rain) scenery(w, h int, seed uint32) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := seeded(seed)
	var ph [6]float64
	for i := range ph {
		ph[i] = r.next() * 2 * math.Pi
	}
	W, H := float64(w), float64(h)
	for x := range w {
		fx := float64(x) / W
		canopy := 0.6 - 0.07*math.Sin(fx*5.1+ph[0]) - 0.05*math.Sin(fx*11.3+ph[1]) - 0.03*math.Sin(fx*23+ph[2])
		canopy -= 0.14 * math.Max(0, math.Sin(fx*2.3+ph[3])) * math.Max(0, math.Sin(fx*7.7+ph[4]))
		ground := 0.86 + 0.015*math.Sin(fx*3.7+ph[5])
		for y := range h {
			fy := float64(y) / H
			var c [3]float64
			switch {
			case fy < canopy:
				k := fy / canopy
				c = [3]float64{0.74 + 0.14*k, 0.77 + 0.13*k, 0.74 + 0.12*k}
			case fy < ground:
				k := (fy - canopy) / (ground - canopy)
				leaf := 0.5 + 0.5*math.Sin(float64(x)*0.21+math.Sin(float64(y)*0.17)*3)
				c = [3]float64{0.1 + 0.05*leaf - 0.03*k, 0.14 + 0.06*leaf - 0.03*k, 0.11 + 0.04*leaf - 0.03*k}
			default:
				k := (fy - ground) / (1 - ground)
				c = [3]float64{0.24 + 0.08*k, 0.3 + 0.08*k, 0.25 + 0.06*k}
			}
			o := img.PixOffset(x, y)
			img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = uint8(255*c[0]), uint8(255*c[1]), uint8(255*c[2]), 255
		}
	}
	return img
}

func boxBlur(img *image.RGBA, radius, passes int) *image.RGBA {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	src := append([]uint8(nil), img.Pix...)
	dst := make([]uint8, len(src))
	for range passes {
		for _, horiz := range [2]bool{true, false} {
			n, m, step, lane := w, h, 4, w*4
			if !horiz {
				n, m, step, lane = h, w, w*4, 4
			}
			glow.Rows(0, m, n, func(ja, jb int) {
				for j := ja; j < jb; j++ {
					base := j * lane
					for ch := range 3 {
						sum, cnt := 0, 0
						for i := 0; i <= radius && i < n; i++ {
							sum += int(src[base+i*step+ch])
							cnt++
						}
						for i := range n {
							dst[base+i*step+ch] = uint8(sum / cnt)
							if a := i - radius; a >= 0 {
								sum -= int(src[base+a*step+ch])
								cnt--
							}
							if b := i + radius + 1; b < n {
								sum += int(src[base+b*step+ch])
								cnt++
							}
						}
					}
					for i := range n {
						dst[base+i*step+3] = 255
					}
				}
			})
			src, dst = dst, src
		}
	}
	out := image.NewRGBA(img.Bounds())
	copy(out.Pix, src)
	return out
}

func rainSize(s, k float64) float64 { return s * (0.0022 + 0.0095*math.Pow(k, 2.6)) }

func (v *rain) add(x, y, r float64) {
	if len(v.drops) >= rainMax {
		return
	}
	for i := range v.drops {
		d := &v.drops[i]
		if !d.run && math.Abs(d.x-x) < d.r+r && math.Abs(d.y-y) < d.r+r && math.Hypot(d.x-x, d.y-y) < d.r+r {
			d.r = math.Sqrt(d.r*d.r + r*r)
			return
		}
	}
	v.drops = append(v.drops, raindrop{x: x, y: y, r: r, phase: v.r.next() * 6})
}

func (v *rain) advance(f frame) {
	W, H := float64(v.st.w), float64(v.st.h)
	s := math.Min(W, H)
	dt := f.dt
	talk := f.state == listening || f.state == responding
	rate := 25.0
	if talk {
		rate = 30 + 160*f.level
	}
	v.spawn += rate * dt
	burst := 0
	if f.onset && talk {
		burst = 6 + int(14*f.level)
	}
	for v.spawn >= 1 || burst > 0 {
		if burst > 0 {
			burst--
		} else {
			v.spawn--
		}
		v.add(v.r.next()*W, v.r.next()*H, rainSize(s, v.r.next()))
	}
	fog := 0.08
	if f.state == responding {
		fog = 0.3 + 0.2*f.level
	}
	v.fog = follow(v.fog, fog, 0.3, 0.1, dt)

	runAt := s * 0.014
	kept := v.drops[:0]
	var trail []raindrop
	for i := range v.drops {
		d := v.drops[i]
		if d.dry {
			if d.r -= dt * s * 0.0006; d.r < s*0.0012 {
				continue
			}
		}
		if !d.run && d.r > runAt {
			d.run = true
		}
		if d.run {
			d.vy = math.Min(d.vy+dt*s*0.9, s*(0.25+18*d.r/s))
			d.y += d.vy * dt
			d.phase += dt * 3
			d.x += math.Sin(d.phase) * s * 0.02 * dt
			d.trail += d.vy * dt
			if d.trail > d.r*3.5 && d.r > runAt*0.55 {
				d.trail = 0
				tr := d.r * (0.18 + 0.1*v.r.next())
				if v.r.next() < 0.55 {
					trail = append(trail, raindrop{x: d.x + (v.r.next()-0.5)*d.r*0.4, y: d.y - d.r*1.4, r: tr, dry: true})
				}
				d.r = math.Sqrt(math.Max(d.r*d.r-tr*tr*0.6, runAt*runAt*0.3))
			}
			if d.y-d.r > H {
				continue
			}
		}
		kept = append(kept, d)
	}
	v.drops = append(kept, trail...)
	for i := range v.drops {
		a := &v.drops[i]
		if !a.run {
			continue
		}
		for j := range v.drops {
			b := &v.drops[j]
			if i == j || b.r == 0 || b.run {
				continue
			}
			if math.Abs(a.x-b.x) < a.r+b.r && math.Abs(a.y-b.y) < a.r+b.r && b.y > a.y-a.r*0.2 && math.Hypot(a.x-b.x, a.y-b.y) < a.r+b.r*0.6 {
				a.r = math.Sqrt(a.r*a.r + b.r*b.r)
				b.r = 0
			}
		}
	}
	kept = v.drops[:0]
	for _, d := range v.drops {
		if d.r > 0 {
			kept = append(kept, d)
		}
	}
	v.drops = kept
}

//go:embed rain.glsl
var rainShader string

func (v *rain) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(rainShader, Light|Feed|FeedHalf|Splat|SplatHalf); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		bw, bh := max(v.st.w/2, 1), max(v.st.h/2, 1)
		sharp := boxBlur(v.scenery(bw, bh, 71), max(bw/90, 1), 2)
		foggy := boxBlur(sharp, max(bw/50, 2), 1)
		for i := 0; i < len(foggy.Pix); i += 4 {
			for c := range 3 {
				foggy.Pix[i+c] = uint8(math.Min(255, float64(foggy.Pix[i+c])*0.85+30))
			}
		}
		if err := g.Texture(0, sharp); err != nil {
			return err
		}
		if err := g.Texture(1, foggy); err != nil {
			return err
		}
		if v.w != v.st.w || v.h != v.st.h {
			v.drops = v.drops[:0]
			W, H := float64(v.st.w), float64(v.st.h)
			s := math.Min(W, H)
			for range 1800 {
				v.add(v.r.next()*W, v.r.next()*H, rainSize(s, v.r.next()))
			}
		}
		v.w, v.h = v.st.w, v.st.h
	}
	f := v.fr.next(x)
	v.advance(f)
	v.pts = v.pts[:0]
	for _, d := range v.drops {
		v.pts = append(v.pts, Point{X: float32(d.x), Y: float32(d.y), Size: float32(2*d.r + 2), A: 1, Splat: true})
		if d.run {
			v.pts = append(v.pts, Point{X: float32(d.x), Y: float32(d.y - d.r*0.9), Size: float32(1.3*d.r + 2), A: 1, Splat: true})
			v.pts = append(v.pts, Point{X: float32(d.x), Y: float32(d.y), Size: float32(3*d.r + 4), R: 0.5, A: 1, Feed: true, Round: true})
		}
	}
	if err := g.Points(v.pts); err != nil {
		return err
	}
	W, H := float64(v.st.w), float64(v.st.h)
	vals := []float32{float32(W), float32(H), float32(v.fog), float32(math.Min(W, H) * 0.06 / W), float32(0.4 + 0.5*f.level)}
	return g.Values(vals, float32(0.2+0.2*f.level), 0.01, 1)
}
