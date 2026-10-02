package visual

import (
	_ "embed"
	"image"
	"math"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/glow"
)

func init() { register(Cymatics, func() Visual { return &cymatics{r: seeded(1787)} }) }

const (
	cymGrains = 9000
	cymGrid   = 192
)

var cymModes = [][3]float64{{1, 2, 1}, {1, 3, -1}, {2, 3, 1}, {1, 4, 1}, {2, 4, -1}, {3, 4, 1}, {1, 5, -1}, {3, 5, 1}, {2, 6, 1}, {4, 5, -1}, {3, 7, 1}, {5, 6, 1}, {4, 7, -1}, {5, 8, 1}}

type cymatics struct {
	st stage
	fr framer
	r  rng

	w, h       int
	gx, gy     []float64
	mode, from int
	blend      float64
	want       int
	wantSince  float64
	centroid   float64
	amp, kick  float64
	idleNext   float64
	pts        []Point
	tables     map[int]*cymTable
}

type cymTable struct{ f, dx, dy []float32 }

func (v *cymatics) table(mode int) *cymTable {
	if t, ok := v.tables[mode]; ok {
		return t
	}
	if v.tables == nil {
		v.tables = map[int]*cymTable{}
	}
	t := &cymTable{make([]float32, cymGrid*cymGrid), make([]float32, cymGrid*cymGrid), make([]float32, cymGrid*cymGrid)}
	m := cymModes[mode]
	for j := range cymGrid {
		for i := range cymGrid {
			f, dx, dy := cymField(m[0], m[1], m[2], (float64(i)+0.5)/cymGrid*2-1, (float64(j)+0.5)/cymGrid*2-1)
			t.f[j*cymGrid+i], t.dx[j*cymGrid+i], t.dy[j*cymGrid+i] = float32(f), float32(dx), float32(dy)
		}
	}
	v.tables[mode] = t
	return t
}

func (t *cymTable) at(x, y float64) (float64, float64, float64) {
	i := min(max(int((x+1)/2*cymGrid), 0), cymGrid-1)
	j := min(max(int((y+1)/2*cymGrid), 0), cymGrid-1)
	k := j*cymGrid + i
	return float64(t.f[k]), float64(t.dx[k]), float64(t.dy[k])
}

func cymField(n, m, sg, x, y float64) (float64, float64, float64) {
	px, py := math.Pi*(x+1)/2, math.Pi*(y+1)/2
	cnx, snx, cmx, smx := math.Cos(n*px), math.Sin(n*px), math.Cos(m*px), math.Sin(m*px)
	cny, sny, cmy, smy := math.Cos(n*py), math.Sin(n*py), math.Cos(m*py), math.Sin(m*py)
	f := cnx*cmy + sg*cmx*cny
	dx := (-n*snx*cmy - sg*m*smx*cny) * math.Pi / 2
	dy := (-m*cnx*smy - sg*n*cmx*sny) * math.Pi / 2
	return f, dx, dy
}

func (v *cymatics) advance(f frame) {
	t, dt := f.t, f.dt
	talk := f.state == listening || f.state == responding
	if talk {
		var sum, wsum float64
		for i, b := range f.bands {
			sum += float64(i) * b
			wsum += b
		}
		if wsum > 0.05 {
			v.centroid = follow(v.centroid, sum/wsum/float64(analysis.Bands-1), 0.6, 0.6, dt)
		}
		idx := int(math.Round(math.Min(1, math.Max(0, (v.centroid-0.12)/0.5)) * float64(len(cymModes)-1)))
		if idx != v.want {
			v.want, v.wantSince = idx, t
		}
		if v.want != v.mode && t-v.wantSince > 0.35 {
			v.from, v.mode, v.blend = v.mode, v.want, 0
		}
	} else if t > v.idleNext {
		v.idleNext = t + 9
		if v.blend >= 1 {
			v.from, v.mode, v.blend = v.mode, (v.mode+1)%5, 0
		}
	}
	v.blend = math.Min(1, v.blend+dt*1.4)
	amp := 0.22
	if talk {
		amp = 0.3 + 1.6*f.level
	}
	v.amp = follow(v.amp, amp, 0.5, 0.25, dt)
	if f.onset && talk {
		v.kick = 1
	}
	v.kick = math.Max(0, v.kick-dt*3)

	a, b := v.table(v.from), v.table(v.mode)
	step := dt * (0.35 + 0.9*v.amp)
	shake := dt * (0.02 + 0.9*v.kick) * (0.4 + v.amp)
	for i := range v.gx {
		x, y := v.gx[i], v.gy[i]
		fv, dx, dy := b.at(x, y)
		if k := v.blend; k < 1 {
			f0, dx0, dy0 := a.at(x, y)
			fv, dx, dy = f0*(1-k)+fv*k, dx0*(1-k)+dx*k, dy0*(1-k)+dy*k
		}
		g := math.Hypot(dx, dy) + 1e-3
		s := math.Copysign(1, fv) * math.Min(math.Abs(fv)*3, 1)
		x -= s * dx / g * step
		y -= s * dy / g * step
		j := math.Abs(fv)*1.2 + 0.22
		x += (v.r.next() - 0.5) * shake * j * 4
		y += (v.r.next() - 0.5) * shake * j * 4
		v.gx[i], v.gy[i] = math.Max(-0.995, math.Min(x, 0.995)), math.Max(-0.995, math.Min(y, 0.995))
	}
}

func (v *cymatics) plate() (cx, cy, half float64) {
	W, H := float64(v.st.w), float64(v.st.h)
	return W / 2, H / 2, math.Min(W, H) * 0.44
}

func (v *cymatics) paint() *image.RGBA {
	w, h := v.st.w, v.st.h
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	cx, cy, half := v.plate()
	u := v.st.u
	glow.Rows(0, h, w, func(ya, yb int) {
		for y := ya; y < yb; y++ {
			for x := range w {
				fx, fy := float64(x)+0.5, float64(y)+0.5
				dx, dy := (fx-cx)/half, (fy-cy)/half
				vig := 1 - 0.5*math.Min(1, (dx*dx+dy*dy)*0.25)
				r, g, b := 0.035*vig, 0.037*vig, 0.042*vig
				edge := math.Max(math.Abs(dx), math.Abs(dy))
				if edge < 1 {
					brush := 0.5 + 0.5*math.Sin(fy*0.9+math.Sin(fx*0.013)*4)
					k := 0.11 + 0.03*brush - 0.04*(dx*dx+dy*dy)*0.5
					r, g, b = k*0.95, k, k*1.06
					rim := (1 - edge) * half
					if rim < 3*u {
						l := 0.1 * (1 - rim/(3*u))
						r, g, b = r+l, g+l, b+l
					}
				} else if edge < 1+6*u/half {
					s := 1 - (edge-1)*half/(6*u)
					r, g, b = r*(1-0.6*s), g*(1-0.6*s), b*(1-0.6*s)
				}
				if d := math.Hypot(fx-cx, fy-cy); d < half*0.045 {
					k := 0.16 + 0.12*(1-d/(half*0.045))
					r, g, b = k, k, k*1.05
				}
				o := img.PixOffset(x, y)
				img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = uint8(255*math.Min(1, r)), uint8(255*math.Min(1, g)), uint8(255*math.Min(1, b)), 255
			}
		}
	})
	return img
}

//go:embed cymatics.glsl
var cymaticsShader string

func (v *cymatics) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(cymaticsShader, Light); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		if err := g.Texture(0, v.paint()); err != nil {
			return err
		}
		if v.gx == nil {
			v.gx, v.gy = make([]float64, cymGrains), make([]float64, cymGrains)
			for i := range v.gx {
				v.gx[i], v.gy[i] = v.r.next()*2-1, v.r.next()*2-1
			}
			v.blend = 1
		}
		v.w, v.h = v.st.w, v.st.h
	}
	f := v.fr.next(x)
	v.advance(f)
	cx, cy, half := v.plate()
	size := float32(math.Max(2, 2.7*v.st.u))
	v.pts = v.pts[:0]
	for i := range v.gx {
		k := float32(0.8 + 0.2*math.Sin(float64(i)*1.7))
		v.pts = append(v.pts, Point{X: float32(cx + v.gx[i]*half), Y: float32(cy + v.gy[i]*half), Size: size,
			R: 0.95 * k * 0.9, G: 0.87 * k * 0.9, B: 0.7 * k * 0.9, A: 0.9, Disc: true, Over: true})
	}
	if err := g.Points(v.pts); err != nil {
		return err
	}
	vals := []float32{float32(v.st.w), float32(cx), float32(cy), float32(half), float32(v.amp)}
	return g.Values(vals, 0, 0, 0)
}
