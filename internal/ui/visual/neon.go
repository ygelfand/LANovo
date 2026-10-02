package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/tfriedel6/canvas"
	"github.com/tfriedel6/canvas/backend/softwarebackend"
	"golang.org/x/image/font/gofont/gomonobold"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/glow"
)

func init() { register(NeonSign, func() Visual { return &neon{r: seeded(1927)} }) }

type neon struct {
	st stage
	fr framer
	r  rng

	w, h      int
	label     string
	letters   int
	bright    float64
	flash     float64
	stutter   int
	stutterOn float64
	nextFail  float64
	tint      [3]float64
}

func (v *neon) wall() *image.RGBA {
	w, h := max(v.st.w/2, 1), max(v.st.h/2, 1)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	s := math.Min(float64(w), float64(h))
	glow.Rows(0, h, w, func(ya, yb int) {
		for y := ya; y < yb; y++ {
			for x := range w {
				r, g, b, _ := brickAt(float64(x)+0.5, float64(y)+0.5, s)
				o := img.PixOffset(x, y)
				img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = uint8(255*r*0.7), uint8(255*g*0.7), uint8(255*b*0.75), 255
			}
		}
	})
	return img
}

func (v *neon) tubes(text string) (*image.RGBA, int) {
	w, h := max(v.st.w/2, 1), max(v.st.h/2, 1)
	W, H := float64(w), float64(h)
	be := softwarebackend.New(w, h)
	cv := canvas.New(be)
	cv.SetFillStyle(color.NRGBA{0, 0, 0, 255})
	cv.FillRect(0, 0, W, H)
	font, _ := cv.LoadFont(gomonobold.TTF)
	words := strings.Fields(text)
	if len(words) == 0 {
		words = []string{"LANOVO"}
	}
	lines := []string{strings.Join(words, " ")}
	if len(words) > 1 && H > W {
		lines = words
	}
	size := math.Min(H*0.34, W*0.3)
	for _, l := range lines {
		cv.SetFont(font, 100)
		size = math.Min(size, W*0.84/math.Max(cv.MeasureText(l).Width, 1)*100)
	}
	size = math.Min(size, H*0.7/float64(len(lines))/1.25)
	cv.SetFont(font, size)
	cv.SetTextAlign(canvas.Left)
	cv.SetTextBaseline(canvas.Middle)
	idx := image.NewRGBA(image.Rect(0, 0, w, h))
	n := 0
	var before []uint8
	for li, l := range lines {
		y := H/2 + (float64(li)-float64(len(lines)-1)/2)*size*1.25
		x := (W - cv.MeasureText(l).Width) / 2
		for _, ch := range l {
			glyph := string(ch)
			adv := cv.MeasureText(glyph).Width
			if ch != ' ' {
				n++
				before = append(before[:0], be.Image.Pix...)
				cv.SetFillStyle(color.NRGBA{255, 255, 255, 255})
				cv.FillText(glyph, x, y)
				for i := 0; i < len(before); i += 4 {
					if be.Image.Pix[i] > before[i] && be.Image.Pix[i] > 40 {
						idx.Pix[i] = uint8(n)
					}
				}
			}
			x += adv
		}
	}
	mask := be.Image
	edge := skeleton(mask)
	plane := make([]uint8, w*h)
	for i := range plane {
		plane[i] = edge.Pix[i*4]
	}
	stroke := max(2, int(size*0.022))
	tube := blurPlane(plane, w, h, stroke, 1)
	glow := blurPlane(plane, w, h, max(2, int(size*0.06)), 2)
	halo := blurPlane(plane, w, h, max(4, int(size*0.45)), 2)
	out := image.NewRGBA(mask.Bounds())
	for i := range plane {
		o := i * 4
		out.Pix[o] = uint8(math.Min(255, float64(tube[i])*float64(2*stroke+1)*0.9))
		out.Pix[o+1] = uint8(math.Min(255, float64(glow[i])*float64(size)*0.12))
		out.Pix[o+2] = uint8(math.Min(255, float64(halo[i])*float64(size)*0.35))
		out.Pix[o+3] = idx.Pix[o]
	}
	return out, n
}

func blurPlane(in []uint8, w, h, radius, passes int) []uint8 {
	src := append([]uint8(nil), in...)
	dst := make([]uint8, len(src))
	for range passes {
		for _, horiz := range [2]bool{true, false} {
			n, m, step, lane := w, h, 1, w
			if !horiz {
				n, m, step, lane = h, w, w, 1
			}
			for j := range m {
				base := j * lane
				sum, cnt := 0, 0
				for i := 0; i <= radius && i < n; i++ {
					sum += int(src[base+i*step])
					cnt++
				}
				for i := range n {
					dst[base+i*step] = uint8(sum / cnt)
					if a := i - radius; a >= 0 {
						sum -= int(src[base+a*step])
						cnt--
					}
					if b := i + radius + 1; b < n {
						sum += int(src[base+b*step])
						cnt++
					}
				}
			}
			src, dst = dst, src
		}
	}
	return src
}

func (v *neon) advance(f frame) {
	t, dt := f.t, f.dt
	talk := f.state == listening || f.state == responding
	want := 0.78 + 0.04*math.Sin(t*1.3)
	if talk {
		want = 0.8 + 0.35*f.level
	}
	v.bright = follow(v.bright, want, 0.5, 0.2, dt)
	if f.onset && talk {
		v.flash = 1
	}
	v.flash = math.Max(0, v.flash-dt*3)
	tint := [3]float64{1, 0.2, 0.62}
	if f.state == responding {
		tint = [3]float64{0.2, 0.85, 1}
	}
	for i := range v.tint {
		v.tint[i] = follow(v.tint[i], tint[i], 0.15, 0.15, dt)
	}
	if t > v.nextFail && v.letters > 0 {
		v.stutter = 1 + int(v.r.next()*float64(v.letters))
		v.nextFail = t + 4 + 8*v.r.next()
		v.stutterOn = t
	}
	v.stutterOn = math.Max(v.stutterOn, 0)
}

func (v *neon) stutterLevel(t float64) float64 {
	age := t - v.stutterOn
	if age > 1.4 || age < 0 {
		return 1
	}
	return map[bool]float64{true: 0.08, false: 1}[math.Sin(age*47)*math.Sin(age*13+1) > 0.1]
}

//go:embed neon.glsl
var neonShader string

func (v *neon) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(neonShader, Light); err != nil {
			return err
		}
	}
	label := x.Label
	if strings.TrimSpace(label) == "" {
		label = "LANOVO"
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		if err := g.Texture(0, v.wall()); err != nil {
			return err
		}
		v.w, v.h, v.label = v.st.w, v.st.h, ""
		v.tint = [3]float64{1, 0.2, 0.62}
	}
	if label != v.label {
		var img *image.RGBA
		withCanvasText(func() { img, v.letters = v.tubes(label) })
		if err := g.Texture(1, img); err != nil {
			return err
		}
		v.label = label
	}
	f := v.fr.next(x)
	v.advance(f)
	buzz := 0.97 + 0.03*math.Sin(f.t*120)
	vals := []float32{float32(v.bright * buzz * (1 + 0.35*v.flash)), float32(v.tint[0]), float32(v.tint[1]), float32(v.tint[2]),
		float32(v.stutter), float32(v.stutterLevel(f.t))}
	return g.Values(vals, float32(0.7+0.4*v.bright), 0.02, 2)
}

func skeleton(mask *image.RGBA) *image.RGBA {
	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()
	x0, y0, x1, y1 := w, h, 0, 0
	on := make([]uint8, w*h)
	for y := range h {
		for x := range w {
			if mask.Pix[mask.PixOffset(x, y)] >= 128 {
				on[y*w+x] = 1
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			}
		}
	}
	out := image.NewRGBA(b)
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = 255
	}
	if x1 < x0 {
		return out
	}
	x0, y0, x1, y1 = max(x0, 1), max(y0, 1), min(x1, w-2), min(y1, h-2)
	var drop []int
	for changed := true; changed; {
		changed = false
		for step := range 2 {
			drop = drop[:0]
			for y := y0; y <= y1; y++ {
				row := y * w
				for x := x0; x <= x1; x++ {
					i := row + x
					if on[i] == 0 {
						continue
					}
					p := [8]uint8{on[i-w], on[i-w+1], on[i+1], on[i+w+1], on[i+w], on[i+w-1], on[i-1], on[i-w-1]}
					n := int(p[0] + p[1] + p[2] + p[3] + p[4] + p[5] + p[6] + p[7])
					if n < 2 || n > 6 {
						continue
					}
					t := 0
					for k := range 8 {
						if p[k] == 0 && p[(k+1)&7] == 1 {
							t++
						}
					}
					if t != 1 {
						continue
					}
					if step == 0 && (p[0]*p[2]*p[4] != 0 || p[2]*p[4]*p[6] != 0) {
						continue
					}
					if step == 1 && (p[0]*p[2]*p[6] != 0 || p[0]*p[4]*p[6] != 0) {
						continue
					}
					drop = append(drop, i)
				}
			}
			for _, k := range drop {
				on[k] = 0
			}
			changed = changed || len(drop) > 0
		}
	}
	for k, v := range on {
		if v != 0 {
			out.Pix[k*4], out.Pix[k*4+1], out.Pix[k*4+2] = 255, 255, 255
		}
	}
	return out
}
