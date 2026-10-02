package visual

import (
	_ "embed"
	"image"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/glow"
)

func init() { register(KoiPond, func() Visual { return newKoi() }) }

const (
	koiCount = 5
	koiSpine = 5
	koiDrops = 4
	koiDiscs = 26
	koiTurn  = 1.1
	koiBend  = 0.42
)

type koiFish struct {
	x, y, heading, speed, sprint, phase, wander float64
	flee, away                                  float64
	spine                                       [koiSpine][2]float64
	length                                      float64
	breed                                       int
	seed                                        float64
}

type koiDrop struct{ x, y, r, s float64 }

type koi struct {
	st stage
	fr framer
	r  rng

	w, h  int
	fish  [koiCount]koiFish
	drops []koiDrop
	next  float64
	born  bool
	pts   []Point
}

func koiProfile(t float64) float64 {
	if t < 0.3 {
		return 0.7 + 0.3*smoothstep(0, 0.3, t)
	}
	return 1 - 0.62*smoothstep(0.3, 1, t)
}

func newKoi() *koi { return &koi{r: seeded(1985)} }

func (v *koi) spawn() {
	W, H := float64(v.st.w), float64(v.st.h)
	s := math.Min(W, H)
	for i := range v.fish {
		f := &v.fish[i]
		f.x, f.y = W*(0.2+0.6*v.r.next()), H*(0.2+0.6*v.r.next())
		f.heading = v.r.next() * 2 * math.Pi
		f.length = s * (0.15 + 0.06*v.r.next())
		f.speed = 0.5 + 0.3*v.r.next()
		f.breed, f.seed = i%5, v.r.next()*50
		f.phase = v.r.next() * 6
		for k := range f.spine {
			f.spine[k] = [2]float64{f.x - math.Cos(f.heading)*f.length*float64(k)/koiSpine, f.y - math.Sin(f.heading)*f.length*float64(k)/koiSpine}
		}
	}
	v.born = true
}

func (v *koi) drop(x, y, strength float64) {
	W, H := float64(v.st.w), float64(v.st.h)
	if len(v.drops) < koiDrops {
		v.drops = append(v.drops, koiDrop{x / W, y / H, 0.011 + 0.006*strength, strength})
	}
	for i := range v.fish {
		f := &v.fish[i]
		dx, dy := f.x-x, f.y-y
		if d := math.Hypot(dx, dy); d < math.Min(W, H)*0.3 {
			f.away, f.flee = math.Atan2(dy, dx), 1.2
			f.sprint = math.Max(f.sprint, 1.5*strength*(1-d/(math.Min(W, H)*0.3)))
		}
	}
}

func (v *koi) advance(f frame) {
	W, H := float64(v.st.w), float64(v.st.h)
	s := math.Min(W, H)
	t, dt := f.t, f.dt
	v.drops = v.drops[:0]
	switch {
	case f.onset && f.state == listening:
		v.drop(W*(0.1+0.8*v.r.next()), H*(0.5+0.42*v.r.next()), 0.45+0.8*f.level)
	case f.onset && f.state == responding:
		v.drop(W*(0.1+0.8*v.r.next()), H*(0.08+0.42*v.r.next()), 0.45+0.8*f.level)
	case f.state == idle && t > v.next:
		v.drop(W*(0.1+0.8*v.r.next()), H*(0.1+0.8*v.r.next()), 0.3+0.2*v.r.next())
	}
	if f.state == idle && t > v.next || v.next-t > 10 {
		v.next = t + 2.5 + 4*v.r.next()
	}
	pace := 1 + 0.6*f.level
	for i := range v.fish {
		k := &v.fish[i]
		k.wander += (v.r.next() - 0.5) * dt * 2.2
		k.wander *= math.Pow(0.3, dt)
		mx, my := W*0.18, H*0.18
		steer := 0.0
		if k.x < mx || k.x > W-mx || k.y < my || k.y > H-my {
			want := math.Atan2(H/2-k.y, W/2-k.x)
			steer = math.Remainder(want-k.heading, 2*math.Pi) * 1.4
		}
		if k.flee > 0 {
			k.flee -= dt
			steer += math.Remainder(k.away-k.heading, 2*math.Pi) * 3
		}
		k.sprint = math.Max(0, k.sprint-dt*0.8)
		sp := s * 0.06 * k.speed * pace * (1 + 2.2*k.sprint)
		k.phase += dt * (2.2 + 5*k.sprint) * pace
		turn := koiTurn * (1 + 0.8*k.sprint)
		k.heading += math.Max(-turn, math.Min(k.wander+steer, turn))*dt + math.Sin(k.phase)*0.012*(1+k.sprint)
		k.x += math.Cos(k.heading) * sp * dt
		k.y += math.Sin(k.heading) * sp * dt
		k.x, k.y = math.Max(W*0.02, math.Min(k.x, W*0.98)), math.Max(H*0.02, math.Min(k.y, H*0.98))
		k.spine[0] = [2]float64{k.x, k.y}
		seg := k.length / (koiSpine - 1)
		ahead := k.heading + math.Pi
		for j := 1; j < koiSpine; j++ {
			dx, dy := k.spine[j][0]-k.spine[j-1][0], k.spine[j][1]-k.spine[j-1][1]
			a := math.Atan2(dy, dx)
			a = ahead + math.Max(-koiBend, math.Min(math.Remainder(a-ahead, 2*math.Pi), koiBend))
			k.spine[j] = [2]float64{k.spine[j-1][0] + math.Cos(a)*seg, k.spine[j-1][1] + math.Sin(a)*seg}
			ahead = a
		}
	}
}

func (v *koi) paintBed() *image.RGBA {
	bw, bh := max(v.st.w/2, 1), max(v.st.h/2, 1)
	img := image.NewRGBA(image.Rect(0, 0, bw, bh))
	cs := math.Min(float64(bw), float64(bh)) * 0.06
	lx, ly, lz := -0.45, -0.55, 0.7
	sr := seeded(5)
	salt := uint32(sr.next() * (1 << 24))
	glow.Rows(0, bh, bw, func(ya, yb int) {
		for y := ya; y < yb; y++ {
			for x := range bw {
				fx, fy := float64(x)+0.5, float64(y)+0.5
				gx, gy := int(math.Floor(fx/cs)), int(math.Floor(fy/cs))
				best, bk, bnx, bny := 2.0, 0.0, 0.0, 0.0
				for oy := -1; oy <= 1; oy++ {
					for ox := -1; ox <= 1; ox++ {
						cx, cy := gx+ox, gy+oy
						h1, h2, h3 := cellHash(cx, cy, salt), cellHash(cx+101, cy-37, salt), cellHash(cx-59, cy+83, salt)
						px, py := (float64(cx)+0.15+0.7*h1)*cs, (float64(cy)+0.15+0.7*h2)*cs
						rr := cs * (0.2 + 0.28*h3)
						if h2 < 0.3 {
							continue
						}
						dx, dy := (fx-px)/rr, (fy-py)/(rr*(0.7+0.3*h1))
						if d := math.Sqrt(dx*dx + dy*dy); d < best {
							best, bk, bnx, bny = d, h3, dx, dy
						}
					}
				}
				mud := 0.5 + 0.5*math.Sin(fx*0.013+math.Sin(fy*0.009)*2)*math.Cos(fy*0.011)
				r, g, b := 16+8*mud, 26+10*mud, 22+6*mud
				if best < 1 {
					nz := math.Sqrt(1 - best*best)
					lit := 0.45 + 0.55*math.Max(0, (bnx*lx+bny*ly+nz*lz)/math.Sqrt(bnx*bnx+bny*bny+nz*nz))
					tone := (30 + 42*bk) * lit
					edge := math.Min(1, (1-best)*cs*0.5)
					r = r + (tone*1.02-r)*edge
					g = g + (tone*0.98-g)*edge
					b = b + (tone*0.82-b)*edge
				}
				o := img.PixOffset(x, y)
				img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = uint8(math.Min(255, r)), uint8(math.Min(255, g)), uint8(math.Min(255, b)), 255
			}
		}
	})
	return img
}

var koiPads = [3][4]float64{{0.2, 0.18, 0.075, 0.6}, {0.82, 0.3, 0.06, 2.4}, {0.72, 0.85, 0.085, 4.1}}

func (v *koi) paintPads(bed *image.RGBA) *image.RGBA {
	W, H := float64(v.st.w), float64(v.st.h)
	s := math.Min(W, H)
	img := image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
	bs := float64(bed.Bounds().Dx()) / W
	shx, shy := s*0.018*0.8, s*0.03*0.8
	for _, p := range koiPads {
		cx, cy, r, notch := W*p[0], H*p[1], s*p[2], p[3]
		for y := max(int(cy-r-2), 0); y < min(int(cy+r+3), v.st.h); y++ {
			for x := max(int(cx-r-2), 0); x < min(int(cx+r+3), v.st.w); x++ {
				dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
				d := math.Hypot(dx, dy)
				ang := math.Atan2(dy, dx)
				if math.Abs(math.Remainder(ang-notch, 2*math.Pi)) < 0.18 {
					continue
				}
				cov := clamp01(r - d + 0.5)
				if cov <= 0 {
					continue
				}
				k := d / r
				vein := 0.9 + 0.1*math.Cos(ang*14)
				gr, gg, gb := (0.18+0.12*k)*vein, (0.42+0.13*k)*vein, (0.16+0.06*k)*vein
				o := img.PixOffset(x, y)
				img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = uint8(255*gr*cov), uint8(255*gg*cov), uint8(255*gb*cov), uint8(255*cov)
			}
		}
		sx, sy, sr := (cx+shx)*bs, (cy+shy)*bs, r*bs
		b := bed.Bounds()
		for y := max(int(sy-sr-2), 0); y < min(int(sy+sr+3), b.Dy()); y++ {
			for x := max(int(sx-sr-2), 0); x < min(int(sx+sr+3), b.Dx()); x++ {
				k := 1 - 0.35*clamp01(sr-math.Hypot(float64(x)+0.5-sx, float64(y)+0.5-sy)+0.5)
				o := bed.PixOffset(x, y)
				bed.Pix[o], bed.Pix[o+1], bed.Pix[o+2] = uint8(float64(bed.Pix[o])*k), uint8(float64(bed.Pix[o+1])*k), uint8(float64(bed.Pix[o+2])*k)
			}
		}
	}
	return img
}

//go:embed koi.glsl
var koiShader string

func (v *koi) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(koiShader, Light|Feed|FeedHalf|FeedFloat|Splat|SplatHalf); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		bed := v.paintBed()
		pads := v.paintPads(bed)
		if err := g.Texture(0, bed); err != nil {
			return err
		}
		if err := g.Texture(1, pads); err != nil {
			return err
		}
		v.w, v.h = v.st.w, v.st.h
		if !v.born {
			v.spawn()
		}
	}
	f := v.fr.next(x)
	v.advance(f)
	W, H := float64(v.st.w), float64(v.st.h)
	s := math.Min(W, H)
	vals := make([]float32, 26)
	vals[0], vals[1] = float32(W), float32(H)
	vals[2], vals[3] = float32(1/math.Max(1, math.Floor(W/2))), float32(1/math.Max(1, math.Floor(H/2)))
	vals[4], vals[5], vals[6], vals[7] = float32(s*0.09), float32(f.t), float32(0.8+0.6*f.level), float32(2.5)
	vals[8], vals[9] = float32(s*0.018), float32(s*0.03)
	for i, d := range v.drops {
		o := 10 + 4*i
		vals[o], vals[o+1], vals[o+2], vals[o+3] = float32(d.x), float32(d.y), float32(d.r*H), float32(d.s)
	}
	v.pts = v.pts[:0]
	shx, shy := float32(s*0.018), float32(s*0.03)
	for i := range v.fish {
		k := &v.fish[i]
		tag := float32(float64(k.breed) + 0.9*math.Mod(k.seed, 1))
		width := k.length * 0.15
		var cum [koiSpine]float64
		for j := 1; j < koiSpine; j++ {
			cum[j] = cum[j-1] + math.Hypot(k.spine[j][0]-k.spine[j-1][0], k.spine[j][1]-k.spine[j-1][1])
		}
		at := func(t float64) (float64, float64, float64, float64) {
			d := t * cum[koiSpine-1]
			j := 1
			for j < koiSpine-1 && cum[j] < d {
				j++
			}
			seg := math.Max(cum[j]-cum[j-1], 1e-6)
			h := clamp01((d - cum[j-1]) / seg)
			a, b := k.spine[j-1], k.spine[j]
			return a[0] + (b[0]-a[0])*h, a[1] + (b[1]-a[1])*h, (b[0] - a[0]) / seg, (b[1] - a[1]) / seg
		}
		add := func(x, y, r float64, t, kind float32, dx, dy float64) {
			head := float32(math.Atan2(dy, dx)/(2*math.Pi) + 0.5)
			v.pts = append(v.pts, Point{X: float32(x), Y: float32(y), Size: float32(2*r + 2), R: t, G: tag, B: head, A: kind, Splat: true})
			if kind > 0.75 && int(t*(koiDiscs-1)+0.5)%3 == 0 {
				v.pts = append(v.pts, Point{X: float32(x) + shx, Y: float32(y) + shy, Size: float32(2*r*1.35 + 2), A: 0, Splat: true})
			}
		}
		for n := 0; n < koiDiscs; n++ {
			t := float64(n) / (koiDiscs - 1)
			x, y, dx, dy := at(t)
			r := width * koiProfile(t)
			add(x, y, r, float32(t), 1, dx, dy)
			if n == 6 {
				for _, sd := range [2]float64{-1, 1} {
					fin := math.Sin(k.phase*1.3+sd) * 0.25
					add(x-dy*sd*r*1.05+dx*r*fin, y+dx*sd*r*1.05+dy*r*fin, r*0.38, 0.35, 0.5, dx, dy)
				}
			}
		}
		tx, ty, dx, dy := at(1)
		sway := math.Sin(k.phase) * 0.35
		cs, sn := math.Cos(sway), math.Sin(sway)
		dx, dy = dx*cs-dy*sn, dx*sn+dy*cs
		for n := 1; n <= 6; n++ {
			ft := float64(n) / 6
			span := 0.12 + 0.36*ft
			if n == 6 {
				span *= 1.15
			}
			for m := -1; m <= 1; m++ {
				if n > 4 && m == 0 {
					continue
				}
				spread := span * float64(m)
				c, sn := math.Cos(spread), math.Sin(spread)
				fx, fy := dx*c-dy*sn, dx*sn+dy*c
				add(tx+fx*width*2.5*ft, ty+fy*width*2.5*ft, width*(0.26+0.28*ft), float32(ft), 0.5, fx, fy)
			}
		}
	}
	if err := g.Points(v.pts); err != nil {
		return err
	}
	return g.Values(vals, float32(0.45+0.3*f.level), 0.015, 2)
}
