package visual

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
	"math"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/ui"
)

func init() { register(Matrix, func() Visual { return &matrix{r: seeded(1999)} }) }

const (
	matrixCols   = 256
	matrixLayers = 2
)

var matrixScale = [matrixLayers]float64{1, 0.6}

//go:embed matrix/glyphs.png
var matrixGlyphs []byte

type stream struct {
	head, tail, speed, wait float64
	rise, jitter            float64
	on                      bool
}

type matrix struct {
	st    stage
	mic   framer
	asst  framer
	r     rng
	w, h  int
	cols  [matrixLayers][]stream
	pour  float64
	state *image.RGBA
	atlas *image.RGBA
}

func (v *matrix) size() float64 { return 28 * v.st.u }

func (v *matrix) layout() {
	s := v.size()
	for l := range matrixLayers {
		n := min(matrixCols, int(math.Ceil(float64(v.st.w)/(s*matrixScale[l]*0.62))))
		v.cols[l] = make([]stream, n)
		for i := range v.cols[l] {
			c := &v.cols[l][i]
			c.jitter = 0.8 + 0.4*v.r.next()
			v.respawn(l, c, true)
		}
	}
}

func (v *matrix) rows(l int) float64 {
	return math.Ceil(float64(v.st.h) / (v.size() * matrixScale[l]))
}

func (v *matrix) respawn(l int, c *stream, first bool) {
	rows := v.rows(l)
	c.speed = (7 + 9*v.r.next()) * (1 - 0.35*float64(l))
	c.tail = math.Round(6 + (rows*0.45)*v.r.next())
	c.head = -1
	if first {
		c.head = math.Floor(v.r.next() * (rows + c.tail))
	}
	c.wait = (0.5 + 5*v.r.next()) * (1 - 0.8*v.pour)
	c.on = v.r.next() < 0.35+0.6*v.pour
}

func (v *matrix) advance(x Input) {
	fm := v.mic.next(Input{Mic: x.Mic, Now: x.Now, Dt: x.Dt})
	fa := v.asst.next(Input{Speaker: x.Speaker, Replying: true, Now: x.Now, Dt: x.Dt})
	dt := fm.dt
	pour := 0.0
	if fa.state != idle {
		pour = fa.level
	}
	v.pour = follow(v.pour, pour, 0.5, 0.1, dt)
	var peak float64
	for _, b := range fm.bands[1:25] {
		peak = math.Max(peak, b)
	}
	speak := 0.0
	if fm.state != idle {
		speak = fm.level
	}
	for l := range matrixLayers {
		rows := v.rows(l)
		n := len(v.cols[l])
		for i := range v.cols[l] {
			c := &v.cols[l][i]
			if c.head < 0 {
				c.wait -= dt
				if c.wait <= 0 {
					c.head = 0
					if !c.on {
						v.respawn(l, c, false)
					}
				}
			} else {
				c.head += c.speed * (1 + 1.4*v.pour) * dt
				if c.head-c.tail > rows {
					v.respawn(l, c, false)
				}
			}
			want := 0.0
			if peak > 0 {
				at := 1 + float64(i)/float64(max(n-1, 1))*23
				b0 := int(at)
				t := at - float64(b0)
				band := fm.bands[b0]*(1-t) + fm.bands[min(b0+1, analysis.Bands-1)]*t
				want = band / peak * speak * c.jitter * rows * 0.85
			}
			c.rise = follow(c.rise, want, 0.5, 0.12, dt)
		}
		if fa.onset && fa.state != idle {
			for range 3 {
				c := &v.cols[l][int(v.r.next()*float64(n))%n]
				c.head, c.on = 0, true
			}
		}
		if fm.onset && fm.state != idle {
			for range 3 {
				c := &v.cols[l][int(v.r.next()*float64(n))%n]
				c.rise += rows * 0.12
			}
		}
	}
}

func (v *matrix) pack() {
	if v.state == nil {
		v.state = image.NewRGBA(image.Rect(0, 0, matrixCols, 4))
	}
	clear(v.state.Pix)
	for l := range matrixLayers {
		for i, c := range v.cols[l] {
			o := v.state.PixOffset(i, l)
			on := c.on && c.head >= 0
			v.state.Pix[o] = uint8(math.Min(255, math.Max(0, math.Floor(c.head))))
			v.state.Pix[o+1] = uint8(math.Min(255, c.tail))
			v.state.Pix[o+2] = uint8(math.Min(255, math.Round(c.rise)))
			if on {
				v.state.Pix[o+3] = 255
			}
		}
	}
}

//go:embed matrix.glsl
var matrixShader string

func (v *matrix) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(matrixShader, Light); err != nil {
			return err
		}
		if v.atlas == nil {
			src, err := png.Decode(bytes.NewReader(matrixGlyphs))
			if err != nil {
				return err
			}
			v.atlas = image.NewRGBA(src.Bounds())
			draw.Draw(v.atlas, v.atlas.Bounds(), src, src.Bounds().Min, draw.Src)
		}
		if err := g.Texture(1, v.atlas); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		v.w, v.h = v.st.w, v.st.h
		v.layout()
	}
	v.advance(x)
	v.pack()
	if err := g.Texture(0, v.state); err != nil {
		return err
	}
	vals := []float32{float32(v.st.w), float32(v.st.h), float32(math.Mod(x.Now.Seconds(), 1000)), float32(v.size()),
		float32(v.rows(0)), float32(v.rows(1))}
	return g.Values(vals, 0.5, 0.025, 2)
}
