package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"
	"strconv"

	"github.com/tfriedel6/canvas"
	"github.com/tfriedel6/canvas/backend/softwarebackend"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"

	"github.com/ygelfand/LANovo/internal/ui"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

// Ported from anchorapp100/ha-visualisations src/26-mother.js (MIT).

func init() { register(Mother, func() Visual { return &mother{r: seeded(6000)} }) }

const motherReady = "INTERFACE 2037 READY FOR INQUIRY"

var motherInk = [3]float64{140, 255, 214}

type lamp struct {
	x, y       float64
	kind       int
	on         bool
	rate, diag float64
	band       int
	flash      float64
}

type mother struct {
	st stage
	fr framer
	r  rng

	w, h           int
	sx, sy, sw, sh float64
	cr, fs, x0, y0 float64
	charW          float64
	lr             float64
	lamps          []lamp
	under, glass   *image.RGBA
	text           *image.RGBA
	waves          [4][200]float64
	ready, trace   *image.RGBA
	line, foot     image.Rectangle
	fy, amp        float64
	typeT          float64
	started        bool
	last           string
	cool, warm     float64
}

func (v *mother) setup() {
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	portrait := Portrait(ui.Rect{W: v.st.w, H: v.st.h})
	cx0, cy0, cw, ch := W*0.03, H*0.05, W*0.555, H*0.9
	lx0, ly0, lw, lh, pc, pr := W*0.615, H*0.05, W*0.355, H*0.9, 3, 5
	if portrait {
		cx0, cy0, cw, ch = W*0.05, H*0.03, W*0.9, H*0.5
		lx0, ly0, lw, lh, pc, pr = W*0.05, H*0.56, W*0.9, H*0.41, 5, 3
	}
	pad := math.Min(cw, ch) * 0.045
	v.sx, v.sy, v.sw, v.sh, v.cr = cx0+pad, cy0+pad, cw-2*pad, ch-2*pad, 16*u
	v.fs = math.Max(10, v.sh*0.042)
	v.x0, v.y0 = v.sx+v.sw*0.07, v.sy+v.sh*0.1
	v.fy, v.amp = v.sy+v.sh*0.9, v.sh*0.05

	be := softwarebackend.New(v.st.w, v.st.h)
	img := be.Image
	cv := canvas.New(be)
	mono, _ := cv.LoadFont(gomono.TTF)
	bold, _ := cv.LoadFont(gomonobold.TTF)
	cv.SetFont(mono, v.fs)
	v.charW = cv.MeasureText("M").Width

	r := seeded(2037)
	gap := math.Min(lw, lh) * 0.03
	pw, ph := (lw-gap*float64(pc-1))/float64(pc), (lh-gap*float64(pr-1))/float64(pr)
	v.lr = math.Min(pw/5, ph/4) * 0.2
	v.lamps = v.lamps[:0]

	fx.Clear(img, hex(0x050607, 1))
	fx.Radial(img, float32(W*0.3), float32(H*0.5), 0, float32(W*0.7), []fx.Stop{{At: 0, C: color.NRGBA{30, 60, 52, 89}}, {At: 1, C: color.NRGBA{}}}, false)
	shadow(img, cx0, cy0, cw, ch, 24*u, 22*u, 0.9)
	rounded(img, cx0, cy0, cw, ch, 24*u, func(sub *image.RGBA, ox, oy float64) {
		fx.LinearRect(img, sub.Bounds(), 0, float32(cy0), 0, float32(cy0+ch), []fx.Stop{{At: 0, C: hex(0x2a2c2b, 1)}, {At: 1, C: hex(0x121313, 1)}}, false)
	})
	rounded(img, v.sx-5*u, v.sy-5*u, v.sw+10*u, v.sh+10*u, v.cr+5*u, func(sub *image.RGBA, ox, oy float64) {
		fillRect(img, ox, oy, float64(sub.Bounds().Dx()), float64(sub.Bounds().Dy()), hex(0x020303, 1))
	})
	rounded(img, v.sx, v.sy, v.sw, v.sh, v.cr, func(sub *image.RGBA, ox, oy float64) {
		fx.Radial(sub, float32(v.sx+v.sw/2-ox), float32(v.sy+v.sh/2-oy), 0, float32(math.Hypot(v.sw, v.sh)/2+1), []fx.Stop{
			{At: 0, C: hex(0x04120e, 1)}, {At: float32(0.75 * v.sw * 0.65 / (math.Hypot(v.sw, v.sh) / 2)), C: hex(0x020a08, 1)}, {At: float32(v.sw * 0.65 / (math.Hypot(v.sw, v.sh) / 2)), C: hex(0x010403, 1)}, {At: 1, C: hex(0x010403, 1)},
		}, false)
	})
	cv.SetTextBaseline(canvas.Middle)
	cv.SetFont(bold, math.Max(10*u, 6))
	cv.SetFillStyle(color.NRGBA{170, 185, 178, 128})
	cv.SetTextAlign(canvas.Left)
	cv.FillText("MU/TH/UR 6000", cx0+24*u, cy0+ch-pad*0.45)
	cv.SetTextAlign(canvas.Right)
	cv.FillText("INTERFACE 2037", cx0+cw-24*u, cy0+ch-pad*0.45)

	for a := range pc {
		for b := range pr {
			px, py := lx0+float64(a)*(pw+gap), ly0+float64(b)*(ph+gap)
			shadow(img, px, py, pw, ph, 5*u, 8*u, 0.8)
			rounded(img, px, py, pw, ph, 5*u, func(sub *image.RGBA, ox, oy float64) {
				fx.LinearRect(img, sub.Bounds(), 0, float32(py), 0, float32(py+ph), []fx.Stop{{At: 0, C: hex(0x1d1f20, 1)}, {At: 1, C: hex(0x111213, 1)}}, false)
			})
			cv.SetStrokeStyle(color.NRGBA{255, 255, 255, 13})
			cv.SetLineWidth(math.Max(1, u))
			roundRect(cv, px+1.5*u, py+1.5*u, pw-3*u, ph-3*u, 4*u)
			cv.Stroke()
			cv.SetFont(bold, math.Max(7.5*u, 5))
			cv.SetFillStyle(color.NRGBA{150, 160, 155, 89})
			cv.SetTextAlign(canvas.Left)
			cv.SetTextBaseline(canvas.Alphabetic)
			cv.FillText(string(rune('A'+b))+"-"+strconv.Itoa(a*pr+b+11), px+5*u, py+ph-6*u)
			for i := range 5 {
				for j := range 4 {
					x, y := px+pw*(0.14+0.72*float64(i)/4), py+ph*(0.16+0.58*float64(j)/3)
					p := r.next()
					kind := 0
					if p >= 0.9 {
						kind = 2
					} else if p >= 0.66 {
						kind = 1
					}
					fx.Radial(img, float32(x), float32(y), 0, float32(v.lr*1.35), []fx.Stop{{At: 0, C: hex(0x070808, 1)}, {At: 1, C: hex(0x070808, 1)}}, false)
					socket := []uint32{0x2c2a26, 0x2e2518, 0x2e1a17}[kind]
					fx.Radial(img, float32(x), float32(y), 0, float32(v.lr), []fx.Stop{{At: 0, C: hex(socket, 1)}, {At: 1, C: hex(socket, 1)}}, false)
					col := a*5 + i
					v.lamps = append(v.lamps, lamp{x: x, y: y, kind: kind, on: r.next() < 0.3, rate: 0.35 + r.next()*1.5,
						band: min(31, int(float64(col)/float64(pc*5-1)*30)), diag: float64(col) + float64(b*4+j)*0.7})
				}
			}
		}
	}

	text := image.NewRGBA(img.Bounds())
	tc := softwarebackend.New(v.st.w, v.st.h)
	tc.Image = text
	tv := canvas.New(tc)
	tv.SetTextBaseline(canvas.Middle)
	tv.SetTextAlign(canvas.Left)
	tv.SetFillStyle(nrgba(motherInk, 0.95))
	tv.SetFont(bold, v.fs)
	tv.FillText("MU/TH/UR 6000", v.x0, v.y0)
	fillRect(text, v.x0, v.y0+v.fs*2.7, v.sw*0.86, math.Max(1, u), nrgba(motherInk, 0.35))
	tv.SetFillStyle(nrgba(motherInk, 0.45))
	tv.SetFont(bold, v.fs*0.62)
	tv.FillText("AUDIO I/O", v.x0, v.fy-v.amp*1.9)
	v.text = text

	v.ready = image.NewRGBA(img.Bounds())
	tc.Image = v.ready
	tv.SetFillStyle(nrgba(motherInk, 0.85))
	tv.SetFont(mono, v.fs)
	tv.FillText(motherReady, v.x0, v.y0+v.fs*1.6)

	v.under = img
	v.glass = image.NewRGBA(img.Bounds())
	v.glassOver()

	bounds := img.Bounds()
	ly := v.y0 + v.fs*1.6
	v.line = image.Rect(int(v.sx), int(ly-v.fs*2.4), int(math.Ceil(v.sx+v.sw)), int(math.Ceil(ly+v.fs*2.4))).Intersect(bounds)
	v.foot = image.Rect(int(v.sx), int(v.fy-v.amp*1.4), int(math.Ceil(v.sx+v.sw)), int(math.Ceil(v.fy+v.amp*1.4))).Intersect(bounds)
	v.trace = image.NewRGBA(v.foot)
	v.w, v.h = v.st.w, v.st.h
}

func (v *mother) glassOver() {
	g := v.glass
	cx, cy := v.sx+v.sw/2, v.sy+v.sh/2
	r0, r1 := v.sh*0.3, v.sw*0.72
	dx, dy := v.sw*0.7, v.sh
	n := dx*dx + dy*dy
	for y := max(int(v.sy), 0); y < min(int(math.Ceil(v.sy+v.sh)), v.st.h); y++ {
		py := float64(y) + 0.5
		scan := 0.0
		if (y-int(v.sy))%3 == 0 {
			scan = 0.22
		}
		for x := max(int(v.sx), 0); x < min(int(math.Ceil(v.sx+v.sw)), v.st.w); x++ {
			px := float64(x) + 0.5
			k := roundCover(px, py, v.sx, v.sy, v.sw, v.sh, v.cr)
			if k <= 0 {
				continue
			}
			vig := 0.6 * clamp01((math.Hypot(px-cx, py-cy)-r0)/(r1-r0))
			t := ((px-v.sx)*dx + (py-v.sy)*dy) / n
			ref := 0.0
			if t < 0.35 {
				ref = 0.05 + (0.012-0.05)*t/0.35
			} else if t < 0.36 {
				ref = 0.012 * (0.36 - t) / 0.01
			}
			dark := 1 - (1-scan)*(1-vig)
			a := (ref + dark*(1-ref)) * k
			c := ref * k
			i := y*g.Stride + x*4
			g.Pix[i], g.Pix[i+1], g.Pix[i+2], g.Pix[i+3] = uint8(c*255), uint8(c*255), uint8(c*255), uint8(a*255)
		}
	}
}

func (v *mother) advance(f frame) {
	t, dt, lv := f.t, f.dt, f.level
	talk := f.state == listening || f.state == responding
	if !v.started {
		v.typeT, v.started, v.last = t-99, true, f.state
	}
	if f.state != v.last {
		if f.state == listening {
			v.typeT = t
		}
		v.last = f.state
	}
	cool, warm := 0.0, 0.0
	switch f.state {
	case listening:
		cool = 1
	case responding:
		warm = 1
	}
	v.cool = follow(v.cool, cool, 0.1, 0.06, dt)
	v.warm = follow(v.warm, warm, 0.1, 0.06, dt)

	act := 0.45
	if talk {
		act = 0.7 + 5.5*lv
	}
	if f.onset && talk {
		for range 7 {
			l := &v.lamps[int(v.r.next()*float64(len(v.lamps)))]
			l.on, l.flash = true, 1
		}
	}
	for i := range v.lamps {
		l := &v.lamps[i]
		k := 1.0
		if talk {
			k = 0.45 + 1.3*f.bands[l.band]
		}
		if v.r.next() < math.Min(dt*l.rate*act*k, 0.9) {
			l.on = !l.on
		}
		l.flash = math.Max(0, l.flash-dt*3)
	}

}

//go:embed mother.glsl
var motherShader string

func (v *mother) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(motherShader, Light); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		withCanvasText(func() { v.setup() })
		text := image.NewRGBA(v.text.Bounds())
		copy(text.Pix, v.text.Pix)
		fx.Add(text, v.ready, 1)
		for unit, img := range map[int]*image.RGBA{0: v.under, 1: v.glass, 2: text} {
			if err := g.Texture(unit, img); err != nil {
				return err
			}
		}
	}
	f := v.fr.next(x)
	t, dt, lv := f.t, f.dt, f.level
	talk := f.state == listening || f.state == responding
	v.advance(f)

	gain := 0.5
	if talk {
		gain = 2.4
	}
	copy(v.waves[1:], v.waves[:3])
	for i := range 200 {
		v.waves[0][i] = math.Tanh(f.wave[int(float64(i)*1.28)] * gain)
	}
	wv := image.NewRGBA(image.Rect(0, 0, 256, 4))
	for r := range 4 {
		for i := range 200 {
			q := uint32(clamp01((v.waves[r][i]+1)/2) * 65535)
			o := wv.PixOffset(i, r)
			wv.Pix[o], wv.Pix[o+1], wv.Pix[o+3] = uint8(q>>8), uint8(q), 255
		}
	}
	if err := g.Texture(3, wv); err != nil {
		return err
	}

	typed := max(0, min(int((t-v.typeT)*34), len(motherReady)))
	cursor := typed < len(motherReady) || math.Mod(t*1.6, 1) < 0.55
	vals := make([]float32, 24)
	vals[0], vals[1], vals[2], vals[3], vals[4] = float32(v.sx), float32(v.sy), float32(v.sw), float32(v.sh), float32(v.cr)
	vals[5] = float32(v.st.w)
	vals[6], vals[7], vals[8] = float32(v.y0+v.fs*1.6), float32(v.fs), float32(v.x0)
	vals[9], vals[10] = float32(float64(typed)*v.charW), float32(v.charW)
	if typed == len(motherReady) {
		vals[9] = float32(v.sw * 2)
	}
	if cursor {
		vals[11] = 1
	}
	vals[13] = float32(0.93 + 0.07*noise1(t*23))
	vals[14], vals[15], vals[16], vals[17] = float32(v.x0), float32(v.sx+v.sw*0.93), float32(v.fy), float32(v.amp)
	vals[18] = float32(v.st.u)
	for r := range 4 {
		vals[20+r] = float32(math.Pow(0.55, float64(r)*dt*30))
	}

	tint, ta := [3]float64{}, 0.0
	if v.cool > 0.02 {
		tint, ta = [3]float64{150, 215, 255}, 0.45*v.cool
	} else if v.warm > 0.02 {
		tint, ta = [3]float64{255, 150, 205}, 0.35*v.warm
	}
	hues := [3][3]float64{{255, 246, 226}, {255, 178, 72}, {255, 74, 44}}
	core := mixc([3]float64{255, 255, 250}, tint, ta)
	pts := make([]Point, 0, 3*len(v.lamps))
	for _, l := range v.lamps {
		k := 0.45 * l.flash
		if l.on {
			k += 0.8
		}
		k = math.Min(k, 1)
		if k < 0.05 {
			continue
		}
		c := mixc(hues[l.kind], tint, ta)
		dot := func(size float64, c [3]float64, a float64, light bool) {
			pts = append(pts, Point{X: float32(l.x), Y: float32(l.y), Size: float32(size),
				R: float32(c[0] / 255), G: float32(c[1] / 255), B: float32(c[2] / 255), A: float32(a), Round: true, Light: light})
		}
		dot(v.lr*8, c, 0.25*k, false)
		dot(v.lr*4.5, c, 0.8*k, true)
		dot(v.lr*1.8, core, k, true)
	}
	if err := g.Points(pts); err != nil {
		return err
	}
	return g.Values(vals, float32(0.75+0.5*lv), 0.02, 2)
}
