package visual

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/tfriedel6/canvas"
	"github.com/tfriedel6/canvas/backend/softwarebackend"
	"golang.org/x/image/font/gofont/gomedium"

	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

// Ported from anchorapp100/ha-visualisations src/28-lcars.js (MIT).

func init() { register(LCARS, func() Visual { return &lcars{r: seeded(47)} }) }

const (
	lcarsBars    = 28
	lcarsSqueeze = 0.72
)

var (
	lcarsOrange = [3]float64{255, 153, 102}
	lcarsPeach  = [3]float64{255, 204, 153}
	lcarsLav    = [3]float64{204, 153, 204}
	lcarsBlue   = [3]float64{153, 153, 255}
	lcarsSky    = [3]float64{153, 204, 255}
	lcarsPink   = [3]float64{255, 153, 204}
	lcarsTan    = [3]float64{204, 153, 102}
)

type lcarsRect struct {
	x, y, w, h float64
	c          [3]float64
	capped     bool
}

type lcars struct {
	st stage
	fr framer
	r  rng

	w, h           int
	th, pitch, pw  float64
	vx0, vx1, vy0  float64
	top, vy1       float64
	blocks, bottom []lcarsRect
	heights        [lcarsBars]float64
	flash          []float64
	numT           float64
	status         string
	text           *canvas.Canvas
	font           *canvas.Font
	paper          *image.RGBA
	labelAt        image.Rectangle
	stripAt        image.Rectangle
}

type lcarsAnchor int

const (
	rightBottom lcarsAnchor = iota
	rightMiddle
	leftMiddle
)

func lcarsText(cv *canvas.Canvas, f *canvas.Font, size float64, at lcarsAnchor, c color.NRGBA, s string, x, y float64) {
	cv.SetFont(f, math.Max(size, 1))
	switch at {
	case rightBottom:
		cv.SetTextAlign(canvas.Right)
		cv.SetTextBaseline(canvas.Bottom)
	case rightMiddle:
		cv.SetTextAlign(canvas.Right)
		cv.SetTextBaseline(canvas.Middle)
	case leftMiddle:
		cv.SetTextAlign(canvas.Left)
		cv.SetTextBaseline(canvas.Middle)
	}
	cv.SetFillStyle(c)
	cv.Save()
	cv.Scale(lcarsSqueeze, 1)
	cv.FillText(s, x/lcarsSqueeze, y)
	cv.Restore()
}

func (v *lcars) setup() {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	r := seeded(1701)
	mx, my, sw := W*0.035, H*0.05, W*0.14
	th := math.Min(H*0.065, W*0.045)
	ro, ri := math.Min(sw*0.85, th*2.6), th*0.9
	yA, yB := my+th+H*0.1, H-my-th-H*0.1
	xE, gapX, gapY := mx+sw+W*0.07, W*0.006, H*0.012
	v.th = th
	v.vx0, v.vx1 = mx+sw+W*0.045, W-mx
	v.vy0 = yA + H*0.04
	v.top, v.vy1 = v.vy0+H*0.05, H-my-th-H*0.05
	v.pitch = (v.vx1 - v.vx0) / lcarsBars
	v.pw = v.pitch * 0.6

	props := []float64{0.22, 0.12, 0.34, 0.14, 0.18}
	cols := [][3]float64{lcarsLav, lcarsPeach, lcarsBlue, lcarsOrange, lcarsTan}
	v.blocks = v.blocks[:0]
	y := yA + gapY
	avail := yB - gapY - y - gapY*float64(len(props)-1)
	for i, p := range props {
		v.blocks = append(v.blocks, lcarsRect{x: mx, y: y, w: sw, h: avail * p, c: cols[i]})
		y += avail*p + gapY
	}
	v.flash = make([]float64, len(v.blocks))

	frame := func(back *image.RGBA) {
		be := softwarebackend.New(v.st.w, v.st.h)
		img := be.Image
		fx.Clear(img, color.NRGBA{0, 0, 0, 255})
		cv := canvas.New(be)
		f, _ := cv.LoadFont(gomedium.TTF)

		elbow := func(top bool, c [3]float64) {
			inside := func(px, d float64) bool {
				if px < mx || px > xE || d < 0 || d > yA-my {
					return false
				}
				if px < mx+ro && d < ro && math.Hypot(px-mx-ro, d-ro) > ro {
					return false
				}
				if px <= mx+sw || d <= th {
					return true
				}
				return px < mx+sw+ri && d < th+ri && math.Hypot(px-mx-sw-ri, d-th-ri) > ri
			}
			depth := func(py float64) float64 {
				if top {
					return py - my
				}
				return H - my - py
			}
			y0, y1 := int(my), int(math.Ceil(yA))
			if !top {
				y0, y1 = int(yB), int(math.Ceil(H-my))
			}
			col := nrgba(c, 1)
			for py := max(y0, 0); py < min(y1, v.st.h); py++ {
				for px := max(int(mx), 0); px < min(int(math.Ceil(xE)), v.st.w); px++ {
					n := 0
					for sy := range 4 {
						for sx := range 4 {
							if inside(float64(px)+(float64(sx)+0.5)/4, depth(float64(py)+(float64(sy)+0.5)/4)) {
								n++
							}
						}
					}
					if n > 0 {
						k := col
						k.A = uint8(255 * n / 16)
						fx.Span(img, py, px, px+1, k, false)
					}
				}
			}
		}
		elbow(true, lcarsOrange)
		elbow(false, lcarsLav)

		for _, b := range v.blocks {
			fillRect(img, b.x, b.y, b.w, b.h, nrgba(b.c, 1))
			label := fmt.Sprintf("%02d-%d", int(r.next()*90+10), 1000+int(r.next()*8999))
			lcarsText(cv, f, math.Min(sw*0.13*1.3, b.h*0.4), rightBottom, color.NRGBA{0, 0, 0, 255}, label, b.x+b.w-b.w*0.06, b.y+b.h-b.h*0.08)
		}

		title := "VOICE INTERFACE"
		cv.SetFont(f, th*1.18)
		tw := cv.MeasureText(title).Width * lcarsSqueeze
		tx := W - mx
		lcarsText(cv, f, th*1.18, rightMiddle, nrgba(lcarsOrange, 1), title, tx, my+th*0.54)

		segs := func(x0, x1, yy, hh float64, parts []float64, cs [][3]float64, capRight bool) []lcarsRect {
			tot := x1 - x0 - gapX*float64(len(parts)-1)
			x := x0
			var out []lcarsRect
			for k, p := range parts {
				w := tot * p
				seg := lcarsRect{x: x, y: yy, w: w, h: hh, c: cs[k], capped: capRight && k == len(parts)-1}
				out = append(out, seg)
				if seg.capped {
					fillRect(img, x, yy, w-hh/2, hh, nrgba(cs[k], 1))
					fx.Radial(img, float32(x+w-hh/2), float32(yy+hh/2), 0, float32(hh/2), []fx.Stop{{At: 0, C: nrgba(cs[k], 1)}, {At: 1, C: nrgba(cs[k], 1)}}, false)
				} else {
					fillRect(img, x, yy, w, hh, nrgba(cs[k], 1))
				}
				x += w + gapX
			}
			return out
		}
		segs(xE+gapX, tx-tw-W*0.015, my, th, []float64{0.46, 0.2, 0.34}, [][3]float64{lcarsLav, lcarsSky, lcarsPeach}, false)
		v.bottom = segs(xE+gapX, W-mx, H-my-th, th, []float64{0.16, 0.42, 0.26, 0.16}, [][3]float64{lcarsBlue, lcarsPeach, lcarsLav, lcarsOrange}, true)

		lcarsText(cv, f, th*0.56, leftMiddle, nrgba(lcarsOrange, 1), "VOICE ANALYSIS", v.vx0, v.vy0)
		fillRect(img, v.vx0, v.vy0+th*0.5, v.vx1-v.vx0, math.Max(1, 1.5*u), nrgba(lcarsPeach, 0.9))
		for i := range lcarsBars {
			cx := float32(v.vx0 + v.pitch*(float64(i)+0.5))
			fx.Polyline(img, []float32{cx, float32(v.top + v.pw/2), cx, float32(v.vy1 - v.pw/2)}, float32(v.pw), color.NRGBA{60, 48, 70, 115}, false)
		}
		copy(back.Pix, img.Pix)
	}
	v.paper = image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
	frame(v.paper)
	size := th * 0.56
	v.labelAt = image.Rect(int(v.vx1-size*6)-1, int(v.vy0-th*0.38)-1, int(math.Ceil(v.vx1))+2, int(math.Ceil(v.vy0+th*0.38))+2).Intersect(v.paper.Bounds())
	v.stripAt = image.Rect(int(xE), int(H-my-th)-1, int(math.Ceil(W-mx))+1, int(math.Ceil(H-my))+1).Intersect(v.paper.Bounds())

	tb := softwarebackend.New(v.st.w, v.st.h)
	tb.Image = v.paper
	v.text = canvas.New(tb)
	v.font, _ = v.text.LoadFont(gomedium.TTF)
	v.status, v.numT = "", -9
	v.w, v.h = v.st.w, v.st.h
}

func (v *lcars) readouts() {
	for k, b := range v.bottom {
		if b.w <= v.th*2.2 {
			continue
		}
		w := b.w
		if b.capped {
			w -= b.h / 2
		}
		fillRect(v.paper, b.x, b.y, w, b.h, nrgba(b.c, 1))
		inset := v.th * 0.25
		if k == len(v.bottom)-1 {
			inset = v.th * 0.7
		}
		s := fmt.Sprintf("%d-%d", int(v.r.next()*90+10), int(v.r.next()*9000+1000))
		lcarsText(v.text, v.font, v.th*0.46*1.3, rightBottom, color.NRGBA{0, 0, 0, 255}, s, b.x+b.w-inset, b.y+b.h-v.th*0.1)
	}
}

func (v *lcars) label(status string, c [3]float64) {
	size := v.th * 0.56
	fillRect(v.paper, v.vx1-size*6, v.vy0-v.th*0.38, size*6+1, v.th*0.76, color.NRGBA{0, 0, 0, 255})
	lcarsText(v.text, v.font, size, rightMiddle, nrgba(c, 1), status, v.vx1, v.vy0)
}

func (v *lcars) advance(f frame) (changed bool) {
	t, dt, lv := f.t, f.dt, f.level
	talk := f.state == listening || f.state == responding

	if f.onset && talk {
		v.flash[int(v.r.next()*float64(len(v.flash)))] = 1
	}
	for i := range v.flash {
		v.flash[i] = math.Max(0, v.flash[i]-dt*3)
	}
	every := 1.4
	if talk {
		every = 0.35
	}
	if t-v.numT > every || t < v.numT {
		v.numT = t
		withCanvasText(v.readouts)
		changed = true
	}
	status, sc := "STANDBY", lcarsTan
	switch f.state {
	case responding:
		status, sc = "TRANSMITTING", lcarsLav
	case listening:
		status, sc = "RECEIVING", lcarsSky
	}
	if status != v.status {
		v.status = status
		withCanvasText(func() { v.label(status, sc) })
		changed = true
	}

	for i := range lcarsBars {
		var target float64
		if talk {
			bi := float64(i) / (lcarsBars - 1) * 30
			b0 := int(bi)
			fr := bi - float64(b0)
			bv := f.bands[b0]*(1-fr) + f.bands[min(31, b0+1)]*fr
			target = clamp01((bv - 0.28) / 0.72 * (0.9 + 0.3*float64(i)/(lcarsBars-1)) * (0.55 + 0.6*lv))
		} else {
			target = 0.05 + 0.05*noise1(float64(i)*0.8+t*1.2)
		}
		h := &v.heights[i]
		if target > *h {
			*h += (target - *h) * (1 - math.Pow(0.25, dt*30))
		} else {
			*h = math.Max(target, *h-dt*1.1)
		}
	}
	return changed
}

//go:embed lcars.glsl
var lcarsShader string

func (v *lcars) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(lcarsShader, Light); err != nil {
			return err
		}
	}
	rebuilt := false
	if fresh || v.w != v.st.w || v.h != v.st.h {
		withCanvasText(func() { v.setup() })
		if err := g.Texture(0, v.paper); err != nil {
			return err
		}
		rebuilt = true
	}
	f := v.fr.next(x)
	lv := f.level
	if v.advance(f) || rebuilt {
		if err := g.Texture(1, crop(v.paper, v.stripAt)); err != nil {
			return err
		}
		if err := g.Texture(2, crop(v.paper, v.labelAt)); err != nil {
			return err
		}
	}
	a, ca, cb := 0.55, lcarsTan, lcarsTan
	switch f.state {
	case responding:
		a, ca, cb = 1, lcarsLav, lcarsPink
	case listening:
		a, ca, cb = 1, lcarsSky, lcarsBlue
	}
	vals := make([]float32, 74)
	for i := range lcarsBars {
		vals[i] = float32(v.heights[i])
	}
	vals[28], vals[29], vals[30], vals[31], vals[32] = float32(v.vx0), float32(v.pitch), float32(v.pw), float32(v.top), float32(v.vy1)
	for k := range 3 {
		vals[33+k], vals[36+k] = float32(ca[k]/255), float32(cb[k]/255)
	}
	vals[39] = float32(a)
	for k, b := range v.blocks {
		if k < 5 {
			vals[40+5*k], vals[41+5*k], vals[42+5*k], vals[43+5*k], vals[44+5*k] = float32(b.x), float32(b.y), float32(b.w), float32(b.h), float32(v.flash[k]*0.55)
		}
	}
	st, lb := v.stripAt, v.labelAt
	vals[65], vals[66], vals[67], vals[68] = float32(st.Min.X), float32(st.Min.Y), float32(st.Dx()), float32(st.Dy())
	vals[69], vals[70], vals[71], vals[72] = float32(lb.Min.X), float32(lb.Min.Y), float32(lb.Dx()), float32(lb.Dy())
	vals[73] = float32(v.st.w)
	return g.Values(vals, float32(0.12+0.2*lv), 0.012, 1)
}
