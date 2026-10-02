package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"
	"sync"

	"github.com/tfriedel6/canvas"
	"github.com/tfriedel6/canvas/backend/softwarebackend"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Ported from anchorapp100/ha-visualisations src/18-phosphor.js (MIT).

func init() { register(Phosphor, func() Visual { return &phosphor{} }) }

type phosphor struct {
	st stage
	fr framer

	w, h           int
	sx, sy, sw, sh float64
	lamps          [2]float64
	ly             float64
	grat           *image.RGBA
	trace          *image.RGBA
	xs, ys         [300]float64
	back           *image.RGBA
	beams          []phosphorBeam
}

type phosphorBeam struct {
	x0, y0, x1, y1, width, a float64
	c                        [3]float64
}

func (v *phosphor) setup() {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	sh := math.Min(H*0.55, W*0.62*0.8)
	sw := sh * 1.25
	sx, sy := (W-sw)/2, H*0.12
	v.sx, v.sy, v.sw, v.sh = sx, sy, sw, sh
	cr := 18 * u

	font := ui.MustLoad(ui.Bold, max(int(11*u*1.4), 7))
	ly := sy + sh + 44*u
	v.ly = ly
	w2, _ := font.Measure("CH2 ASSIST")
	w1, _ := font.Measure("CH1 MIC")
	x2 := sx + sw + 10*u
	l2 := x2 - float64(w2) - 12*u
	x1 := l2 - 26*u
	v.lamps = [2]float64{x1 - float64(w1) - 12*u, l2}

	housing := func(back *image.RGBA) {
		be := softwarebackend.New(v.st.w, v.st.h)
		img := be.Image
		fx.LinearRect(img, img.Bounds(), 0, 0, 0, float32(H), []fx.Stop{{At: 0, C: hex(0x1b1d1f, 1)}, {At: 1, C: hex(0x0c0d0e, 1)}}, false)
		r := seeded(2)
		for range 1800 {
			fx.Square(img, float32(r.next()*W), float32(r.next()*H), float32(math.Max(u, 0.6)), color.NRGBA{255, 255, 255, uint8(r.next() * 0.025 * 255)}, false)
		}
		cv := canvas.New(be)
		cv.SetShadowColor(color.NRGBA{0, 0, 0, 230})
		cv.SetShadowBlur(24 * u)
		bz := cv.CreateLinearGradient(0, sy-26*u, 0, sy+sh+26*u)
		bz.AddColorStop(0, color.NRGBA{42, 45, 48, 255})
		bz.AddColorStop(1, color.NRGBA{19, 20, 22, 255})
		cv.SetFillStyle(bz)
		roundRect(cv, sx-26*u, sy-26*u, sw+52*u, sh+52*u, 30*u)
		cv.Fill()
		cv.SetShadowBlur(0)
		cv.SetShadowColor(color.NRGBA{})
		cv.SetFillStyle(color.NRGBA{5, 6, 6, 255})
		roundRect(cv, sx-6*u, sy-6*u, sw+12*u, sh+12*u, cr+6*u)
		cv.Fill()
		cv.Save()
		roundRect(cv, sx, sy, sw, sh, cr)
		cv.Clip()
		vg := cv.CreateRadialGradient(sx+sw/2, sy+sh/2, 0, sx+sw/2, sy+sh/2, sw*0.62)
		vg.AddColorStop(0, color.NRGBA{7, 19, 12, 255})
		vg.AddColorStop(0.7, color.NRGBA{4, 11, 7, 255})
		vg.AddColorStop(1, color.NRGBA{1, 3, 2, 255})
		cv.SetFillStyle(vg)
		cv.FillRect(sx, sy, sw, sh)
		cv.Restore()
		surf := rgbaSurface{img}
		ink, bg := theme.Color{R: 125, G: 138, B: 128}, theme.Color{R: 14, G: 15, B: 16}
		_, th := font.Measure("M")
		ui.DrawText(surf, font, int(sx-10*u), int(ly)-th/2, ink, bg, "ASSIST·SCOPE  LS-1919")
		ui.DrawText(surf, font, int(x2)-w2, int(ly)-th/2, ink, bg, "CH2 ASSIST")
		ui.DrawText(surf, font, int(x1)-w1, int(ly)-th/2, ink, bg, "CH1 MIC")
		copy(back.Pix, img.Pix)
	}
	v.back = image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
	var wg sync.WaitGroup
	wg.Go(func() { housing(v.back) })
	defer wg.Wait()

	gb := softwarebackend.New(v.st.w, v.st.h)
	c := canvas.New(gb)
	roundRect(c, sx, sy, sw, sh, cr)
	c.Clip()
	dx, dy := sw/10, sh/8
	c.SetStrokeStyle(color.NRGBA{120, 200, 150, 56})
	c.SetLineWidth(math.Max(0.5, 0.9*u))
	for i := 1; i < 10; i++ {
		c.BeginPath()
		c.MoveTo(sx+float64(i)*dx, sy)
		c.LineTo(sx+float64(i)*dx, sy+sh)
		c.Stroke()
	}
	for i := 1; i < 8; i++ {
		c.BeginPath()
		c.MoveTo(sx, sy+float64(i)*dy)
		c.LineTo(sx+sw, sy+float64(i)*dy)
		c.Stroke()
	}
	c.SetStrokeStyle(color.NRGBA{120, 200, 150, 77})
	for i := 1; i < 50; i++ {
		tx := sx + float64(i)*sw/50
		c.BeginPath()
		c.MoveTo(tx, sy+sh/2-3*u)
		c.LineTo(tx, sy+sh/2+3*u)
		c.Stroke()
	}
	for i := 1; i < 40; i++ {
		ty := sy + float64(i)*sh/40
		c.BeginPath()
		c.MoveTo(sx+sw/2-3*u, ty)
		c.LineTo(sx+sw/2+3*u, ty)
		c.Stroke()
	}
	gl := c.CreateLinearGradient(sx, sy, sx+sw*0.6, sy+sh)
	gl.AddColorStop(0, color.NRGBA{255, 255, 255, 18})
	gl.AddColorStop(0.4, color.NRGBA{255, 255, 255, 4})
	gl.AddColorStop(0.41, color.NRGBA{255, 255, 255, 0})
	c.SetFillStyle(gl)
	c.FillRect(sx, sy, sw, sh)
	ed := c.CreateRadialGradient(sx+sw/2, sy+sh/2, sh*0.35, sx+sw/2, sy+sh/2, sw*0.7)
	ed.AddColorStop(0, color.NRGBA{0, 0, 0, 0})
	ed.AddColorStop(1, color.NRGBA{0, 0, 0, 140})
	c.SetFillStyle(ed)
	c.FillRect(sx, sy, sw, sh)
	v.grat = gb.Image
	v.trace = image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h))
	v.w, v.h = v.st.w, v.st.h
}

func (v *phosphor) beam(f frame) {
	u := v.st.u
	t, lv := f.t, f.level
	sw, sh, sx, sy := v.sw, v.sh, v.sx, v.sy
	midX, midY := sx+sw/2, sy+sh/2
	wv := f.wave
	v.beams = v.beams[:0]
	n := 0
	switch f.state {
	case listening:
		trig, best := 0, 0.0
		for i := range 90 {
			if sl := wv[i+1] - wv[i]; wv[i] <= 0 && wv[i+1] > 0 && sl > best {
				best, trig = sl, i
			}
		}
		for i := range 160 {
			v.xs[n], v.ys[n] = sx+float64(i)/159*sw, midY-math.Tanh(wv[min(trig+i, analysis.WavePoints-1)]*2.2)*sh*0.36
			n++
		}
	case responding:
		rot := t * 0.35
		cr, sr := math.Cos(rot), math.Sin(rot)
		for i := range 256 {
			a, b := math.Tanh(wv[i]*3.2), math.Tanh(wv[(i+37)&255]*3.2)
			v.xs[n], v.ys[n] = midX+(a*cr-b*sr)*sh*0.4, midY-(a*sr+b*cr)*sh*0.4
			n++
		}
	default:
		for i := range 128 {
			v.xs[n], v.ys[n] = sx+float64(i)/127*sw, midY+(noise1(float64(i)*0.9+t*20)-0.5)*2.4*u
			n++
		}
	}
	alphas := [5]float64{0.22, 0.4, 0.62, 0.85, 1}
	boost := 0.75 + 0.35*lv
	k0 := 6.6 * u
	for i := 1; i < n; i++ {
		dx, dy := v.xs[i]-v.xs[i-1], v.ys[i]-v.ys[i-1]
		inten := k0 / (math.Sqrt(dx*dx+dy*dy)*3 + 0.5)
		bk := 0
		switch {
		case inten > 0.9:
			bk = 4
		case inten > 0.5:
			bk = 3
		case inten > 0.25:
			bk = 2
		case inten > 0.1:
			bk = 1
		}
		c := [3]float64{90, 255, 150}
		if bk > 3 {
			c = [3]float64{190, 255, 215}
		}
		v.beams = append(v.beams, phosphorBeam{v.xs[i-1], v.ys[i-1], v.xs[i], v.ys[i], (1.8 + float64(bk)*0.25) * u, math.Min(1, alphas[bk]*boost), c})
	}

}

//go:embed phosphor.glsl
var phosphorShader string

func (v *phosphor) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(phosphorShader, Light|Feed|Lines); err != nil {
			return err
		}
	}
	if fresh || v.back == nil || v.w != v.st.w || v.h != v.st.h {
		v.setup()
		if err := g.Texture(0, v.back); err != nil {
			return err
		}
		if err := g.Texture(1, v.grat); err != nil {
			return err
		}
	}
	f := v.fr.next(x)
	u := v.st.u
	v.beam(f)
	segs := make([]Segment, 0, len(v.beams))
	for _, b := range v.beams {
		segs = append(segs, Segment{X0: float32(b.x0), Y0: float32(b.y0), X1: float32(b.x1), Y1: float32(b.y1), Width: float32(b.width),
			R: float32(b.c[0] / 255), G: float32(b.c[1] / 255), B: float32(b.c[2] / 255), A: float32(b.a)})
	}
	if err := g.Lines(segs); err != nil {
		return err
	}
	pts := make([]Point, 0, 4)
	for j, on := range []bool{f.state == listening, f.state == responding} {
		lx, rr := v.lamps[j], 4*u
		if on {
			pts = append(pts, Point{X: float32(lx), Y: float32(v.ly), Size: float32(rr * 8), R: 80.0 / 255, G: 1, B: 140.0 / 255, A: 0.8, Round: true, Light: true},
				Point{X: float32(lx), Y: float32(v.ly), Size: float32(rr * 2), R: 170.0 / 255, G: 1, B: 200.0 / 255, A: 1, Round: true})
		} else {
			pts = append(pts, Point{X: float32(lx), Y: float32(v.ly), Size: float32(math.Max(rr*2, 3)), R: 40.0 / 255, G: 70.0 / 255, B: 50.0 / 255, A: 1, Disc: true, Over: true})
		}
	}
	if err := g.Points(pts); err != nil {
		return err
	}
	vals := []float32{float32(math.Pow(0.7, f.dt*30)), float32(v.sx), float32(v.sy), float32(v.sw), float32(v.sh), float32(18 * u), float32(v.st.w)}
	return g.Values(vals, float32(1+0.5*f.level), 0.022, 2)
}
