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
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Ported from anchorapp100/ha-visualisations src/25-hal.js (MIT).

func init() { register(HAL9000, func() Visual { return &hal{i: 0.5} }) }

type hal struct {
	st stage
	fr framer

	i, flick, cool float64
	w, h           int
	cx, cy         float64
	rc, rg         float64
	px, pw         float64
}

func (v *hal) geometry() {
	W, H := float64(v.st.w), float64(v.st.h)
	pw := math.Min(W*0.34, H*0.52)
	if Portrait(ui.Rect{W: v.st.w, H: v.st.h}) {
		pw = math.Min(W*0.62, H*0.4)
	}
	v.pw, v.px = pw, W/2-pw/2
	v.cx, v.cy = W/2, H*0.53
	v.rc = pw * 0.41
	v.rg = v.rc * 0.75
	v.w, v.h = v.st.w, v.st.h
}

func (v *hal) paintBack(back *image.RGBA) {
	H, u := float64(v.st.h), v.st.u
	cx, cy, px, pw, rc, rg := v.cx, v.cy, v.px, v.pw, v.rc, v.rg
	rb := rc * 0.87
	be := softwarebackend.New(v.st.w, v.st.h)
	img := be.Image
	fx.Clear(img, color.NRGBA{3, 3, 4, 255})
	fx.Radial(img, float32(cx), float32(cy), 0, float32(H*1.1), []fx.Stop{{At: 0, C: color.NRGBA{46, 46, 52, 128}}, {At: 1, C: color.NRGBA{}}}, false)
	shadow(img, px, -30, pw, H+60, 0, 34*u, 0.9)
	fx.LinearRect(img, image.Rect(int(px), 0, int(px+pw), v.st.h), float32(px), 0, float32(px+pw), 0, []fx.Stop{
		{At: 0, C: hex(0x63686d, 1)}, {At: 0.16, C: hex(0xa2a7ab, 1)}, {At: 0.5, C: hex(0xc6c9cc, 1)}, {At: 0.84, C: hex(0x9ea3a7, 1)}, {At: 1, C: hex(0x5d6267, 1)},
	}, false)
	r := seeded(2001)
	stroke := max(1, int(math.Round(0.6*u)))
	for range 1100 {
		a := r.next() * 0.07
		c := color.NRGBA{0, 0, 0, uint8(a * 255)}
		if r.next() < 0.5 {
			c = color.NRGBA{255, 255, 255, uint8(a * 255)}
		}
		x0 := px + r.next()*pw
		y0 := -30 + r.next()*H*0.4
		y1 := y0 + H*(0.3+r.next()*0.9)
		for y := max(int(y0), 0); y < min(int(y1), v.st.h); y++ {
			fx.Span(img, y, int(x0), int(x0)+stroke, c, false)
		}
	}
	fx.LinearRect(img, image.Rect(int(px), 0, int(px+pw), v.st.h), 0, 0, 0, float32(H), []fx.Stop{
		{At: 0, C: color.NRGBA{255, 255, 255, 26}}, {At: 0.5, C: color.NRGBA{}}, {At: 1, C: color.NRGBA{0, 0, 0, 102}},
	}, false)
	fillRect(img, px, 0, math.Max(1, u), H, color.NRGBA{255, 255, 255, 82})
	fillRect(img, px+pw-math.Max(1, 1.5*u), 0, math.Max(1, 1.5*u), H, color.NRGBA{0, 0, 0, 128})

	nw, nh := pw*0.66, pw*0.15
	nx, ny := cx-nw/2, H*0.1
	shadow(img, nx, ny+2*u, nw, nh, 3*u, 7*u, 0.65)
	fillRect(img, nx, ny, nw, nh, color.NRGBA{10, 11, 12, 255})
	fs := max(int(nh*0.5), 6)
	bold, thin := ui.MustLoad(ui.Bold, fs), ui.MustLoad(ui.Regular, fs)
	w1, th := bold.Measure("HAL")
	w2, _ := thin.Measure(" 9000")
	tx := int(cx) - (w1+w2)/2
	ty := int(ny+nh*0.53) - th/2
	surf := rgbaSurface{img}
	ui.DrawText(surf, bold, tx, ty, theme.Color{R: 241, G: 241, B: 241}, theme.Color{R: 10, G: 11, B: 12}, "HAL")
	ui.DrawText(surf, thin, tx+w1, ty, theme.Color{R: 241, G: 241, B: 241}, theme.Color{R: 10, G: 11, B: 12}, " 9000")

	shadow(img, cx-rc, cy-rc+4*u, 2*rc, 2*rc, rc, 14*u, 0.65)
	conic(img, cx, cy, rc, -0.7, []fx.Stop{
		{At: 0, C: hex(0xeceef0, 1)}, {At: 0.12, C: hex(0x8a8e93, 1)}, {At: 0.26, C: hex(0xf6f7f8, 1)}, {At: 0.4, C: hex(0x6a6e73, 1)},
		{At: 0.55, C: hex(0xdadcdf, 1)}, {At: 0.7, C: hex(0x777b80, 1)}, {At: 0.86, C: hex(0xeff0f2, 1)}, {At: 1, C: hex(0xeceef0, 1)},
	})
	fx.Ring(img, float32(cx), float32(cy), float32(rc*0.955), float32(math.Max(1, 1.3*u)), color.NRGBA{0, 0, 0, 89}, false)
	fx.Ring(img, float32(cx), float32(cy), float32(rc-0.8*u), float32(math.Max(1, 0.8*u)), color.NRGBA{255, 255, 255, 89}, false)
	fx.Radial(img, float32(cx), float32(cy), 0, float32(rb), []fx.Stop{{At: 0, C: hex(0x202023, 1)}, {At: 1, C: hex(0x040404, 1)}}, false)
	fx.Radial(img, float32(cx), float32(cy), 0, float32(rg+1.5*u), []fx.Stop{{At: 0, C: hex(0, 1)}, {At: 1, C: hex(0, 1)}}, false)
	copy(back.Pix, img.Pix)
}

func (v *hal) paintGlass() *image.RGBA {
	u, cx, cy, rg := v.st.u, v.cx, v.cy, v.rg
	gb := softwarebackend.New(v.st.w, v.st.h)
	q := canvas.New(gb)
	q.BeginPath()
	q.Arc(cx, cy, rg, 0, 2*math.Pi, false)
	q.Clip()
	for j, k := range []float64{0.2, 0.33, 0.47, 0.6, 0.74, 0.88} {
		q.BeginPath()
		q.Arc(cx, cy, rg*k, 0, 2*math.Pi, false)
		q.SetStrokeStyle(color.NRGBA{255, 255, 255, uint8((0.028 + 0.014*float64(j%2)) * 255)})
		q.SetLineWidth(math.Max(1, u))
		q.Stroke()
	}
	arc := func(rad, a0, a1, w, a float64) {
		q.BeginPath()
		q.Arc(cx, cy, rad, a0, a1, false)
		q.SetStrokeStyle(color.NRGBA{255, 255, 255, uint8(a * 255)})
		q.SetLineWidth(w)
		q.SetLineCap(canvas.Round)
		q.Stroke()
	}
	arc(rg*0.8, math.Pi*1.07, math.Pi*1.38, 4*u, 0.12)
	arc(rg*0.69, math.Pi*1.11, math.Pi*1.31, 2.4*u, 0.08)
	arc(rg*0.87, math.Pi*1.56, math.Pi*1.72, 2*u, 0.05)
	arc(rg*0.55, math.Pi*0.2, math.Pi*0.32, 1.6*u, 0.04)
	for i := range 6 {
		aa, rr := math.Pi*(1.15+float64(i)*0.034), rg*0.6
		q.BeginPath()
		q.Arc(cx+math.Cos(aa)*rr, cy+math.Sin(aa)*rr, (1.1+0.7*float64(i%2))*u, 0, 2*math.Pi, false)
		q.SetFillStyle(color.NRGBA{255, 255, 255, 66})
		q.Fill()
	}
	sp := q.CreateRadialGradient(cx-rg*0.38, cy-rg*0.43, 0, cx-rg*0.38, cy-rg*0.43, rg*0.17)
	sp.AddColorStop(0, color.NRGBA{255, 255, 255, 140})
	sp.AddColorStop(0.35, color.NRGBA{255, 255, 255, 31})
	sp.AddColorStop(1, color.NRGBA{255, 255, 255, 0})
	q.SetFillStyle(sp)
	q.FillRect(cx-rg, cy-rg, rg*2, rg*2)
	sheen := q.CreateLinearGradient(0, cy-rg, 0, cy+rg*0.15)
	sheen.AddColorStop(0, color.NRGBA{255, 255, 255, 18})
	sheen.AddColorStop(1, color.NRGBA{255, 255, 255, 0})
	q.SetFillStyle(sheen)
	q.FillRect(cx-rg, cy-rg, rg*2, rg*2)
	ed := q.CreateRadialGradient(cx, cy, rg*0.6, cx, cy, rg)
	ed.AddColorStop(0, color.NRGBA{0, 0, 0, 0})
	ed.AddColorStop(1, color.NRGBA{0, 0, 0, 178})
	q.SetFillStyle(ed)
	q.FillRect(cx-rg, cy-rg, rg*2, rg*2)
	return gb.Image
}

func (v *hal) glassAt() image.Rectangle {
	return image.Rect(int(v.cx-v.rg-2), int(v.cy-v.rg-2), int(v.cx+v.rg+3), int(v.cy+v.rg+3)).Intersect(image.Rect(0, 0, v.st.w, v.st.h))
}

func conic(img *image.RGBA, cx, cy, r, start float64, stops []fx.Stop) {
	b := img.Bounds()
	fx.Rows(max(int(cy-r), 0), min(int(cy+r)+1, b.Dy()), int(2*r), func(ya, yb int) {
		for y := ya; y < yb; y++ {
			for x := max(int(cx-r), 0); x < min(int(cx+r)+1, b.Dx()); x++ {
				dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
				d := math.Hypot(dx, dy)
				if d > r {
					continue
				}
				a := math.Mod(math.Atan2(dy, dx)-start+4*math.Pi, 2*math.Pi) / (2 * math.Pi)
				c := gradAt(stops, a)
				c.A = uint8(255 * math.Min(1, r-d+0.5))
				fx.Span(img, y, x, x+1, c, false)
			}
		}
	})
}

func (v *hal) advance(f frame) float64 {
	talk := f.state == listening || f.state == responding
	if f.onset && talk {
		v.flick = 1
	}
	v.flick = math.Max(0, v.flick-f.dt*5)
	var target float64
	switch f.state {
	case responding:
		target = 0.5 + 0.8*f.level
	case listening:
		target = 0.48 + 0.3*f.level
	default:
		target = 0.46 + 0.04*math.Sin(f.t*0.9)
	}
	v.i = follow(v.i, target, 0.35, 0.12, f.dt)
	cool := 0.0
	if f.state == listening {
		cool = 1
	}
	v.cool = follow(v.cool, cool, 0.1, 0.06, f.dt)
	flickBy := 0.1
	if f.state == responding {
		flickBy = 0.22
	}
	return v.i + flickBy*v.flick
}

//go:embed hal.glsl
var halShader string

func (v *hal) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(halShader, Light); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		v.geometry()
		back := image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
		v.paintBack(back)
		if err := g.Texture(0, back); err != nil {
			return err
		}
		if err := g.Texture(1, crop(v.paintGlass(), v.glassAt())); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	intensity := v.advance(f)
	cool := 0.0
	if v.cool > 0.02 {
		cool = (0.08 + 0.3*f.level) * v.cool
	}
	ga := v.glassAt()
	vals := []float32{float32(v.cx), float32(v.cy), float32(v.rg), float32(v.rc), float32(intensity),
		float32(math.Max(0.5, math.Min(1.4, 0.72+0.5*intensity))), float32(clamp01(0.5 + 0.55*intensity)), float32(cool),
		float32(v.st.u), float32(v.st.w), float32(ga.Min.X), float32(ga.Min.Y), float32(ga.Dx()), float32(ga.Dy())}
	return g.Values(vals, float32(0.65+0.7*intensity), 0.03, 2)
}
