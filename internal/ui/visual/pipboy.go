package visual

import (
	_ "embed"
	"image"
	"image/color"
	"math"
	"sync"

	"github.com/tfriedel6/canvas"
	"github.com/tfriedel6/canvas/backend/softwarebackend"
	"golang.org/x/image/font/opentype"

	"github.com/ygelfand/LANovo/internal/lib/analysis"
	"github.com/ygelfand/LANovo/internal/ui"
)

const (
	pipSamples = 128
	pipSlope   = 40
	pipFloor   = 0.04
	pipMicHold = 2.0
)

type pipStyle struct {
	tint           [3]float64
	warp           float64
	scan, scanDeep float64
	roll           float64
	tabs           []string
	tab            int
	subtabs        []string
	subtab         int
	status         bool
	footer         string
	stations       []string
}

var pipStyles = map[Kind]pipStyle{
	PipBoy3: {
		tint: [3]float64{0.62, 1, 0.36}, warp: 0.075, scan: 3, scanDeep: 0.32, roll: 1,
		tabs: []string{"STATS", "ITEMS", "DATA"}, tab: 2,
		subtabs: []string{"Local Map", "World Map", "Quests", "Misc", "Radio"}, subtab: 4,
		stations: []string{"Microphone", "Assistant", "Wasteland Wireless", "Capital Beacon", "Vault Emergency Channel"},
	},
	PipBoyNV: {
		tint: [3]float64{1, 0.71, 0.26}, warp: 0.075, scan: 3, scanDeep: 0.3, roll: 1,
		tabs: []string{"STATS", "ITEMS", "DATA"}, tab: 2,
		subtabs: []string{"Local Map", "World Map", "Quests", "Misc", "Radio"}, subtab: 4,
		stations: []string{"Microphone", "Assistant", "Desert Serenade", "Lucky Frequency", "Radio Neon Strip"},
	},
	PipBoy4: {
		tint: [3]float64{0.1, 1, 0.5}, warp: 0.03, scan: 4, scanDeep: 0.14, roll: 0.4,
		tabs: []string{"STAT", "INV", "DATA", "MAP", "RADIO"}, tab: 4,
		footer:   "10.23.2287          09:47 AM",
		stations: []string{"Microphone", "Assistant", "Harbor Swing", "Old North Relay", "Distress Signal"},
	},
	PipBoy76: {
		tint: [3]float64{0.55, 1, 0.72}, warp: 0.05, scan: 3.5, scanDeep: 0.22, roll: 0.7,
		tabs: []string{"STAT", "INV", "DATA", "MAP", "RADIO"}, tab: 4, status: true,
		stations: []string{"Microphone", "Assistant", "Hollow Ridge Radio", "Mountain Relay", "Vault Call-In"},
	},
}

func init() {
	for k := range pipStyles {
		register(k, func() Visual { return &pipboy{kind: k, active: -1} })
	}
}

type pipboy struct {
	st   stage
	fr   framer
	kind Kind

	w, h      int
	fs, rowH  float64
	list, box image.Rectangle
	wave      [pipSamples]float64
	peak      float64
	active    int
	since     float64
	heard     float64
}

func (v *pipboy) layout() {
	W, H := float64(v.st.w), float64(v.st.h)
	s := math.Min(W, H)
	v.fs = s * 0.042
	v.rowH = v.fs * 1.7
	n := len(pipStyles[v.kind].stations)
	top := H * 0.2
	if Portrait(ui.Rect{W: v.st.w, H: v.st.h}) {
		v.list = image.Rect(int(W*0.08), int(top), int(W*0.92), int(top+v.rowH*float64(n)))
		v.box = image.Rect(int(W*0.08), int(H*0.52), int(W*0.92), int(H*0.82))
	} else {
		v.list = image.Rect(int(W*0.07), int(top), int(W*0.45), int(top+v.rowH*float64(n)))
		v.box = image.Rect(int(W*0.52), int(top), int(W*0.93), int(H*0.78))
	}
	v.w, v.h = v.st.w, v.st.h
}

func (v *pipboy) paint(highlight bool) *image.RGBA {
	sty := pipStyles[v.kind]
	W, H, u := float64(v.st.w), float64(v.st.h), v.st.u
	be := softwarebackend.New(v.st.w, v.st.h)
	cv := canvas.New(be)
	mono, bold := monoFaces()
	white := color.NRGBA{255, 255, 255, 255}
	dim := color.NRGBA{255, 255, 255, 150}
	lw := math.Max(1.5, 2*u)
	cv.SetLineWidth(lw)
	line := func(c color.NRGBA, pts ...float64) {
		cv.SetStrokeStyle(c)
		cv.BeginPath()
		cv.MoveTo(pts[0], pts[1])
		for i := 2; i < len(pts); i += 2 {
			cv.LineTo(pts[i], pts[i+1])
		}
		cv.Stroke()
	}
	text := func(s string, x, y float64, f *opentype.Font, size, anchor float64, c color.NRGBA) float64 {
		return letterAt(be.Image, f, size, c, s, x, y, anchor)
	}
	fit := func(s string, f *opentype.Font, size, room float64) float64 {
		if w := measure(f, size, s); w > room {
			return size * room / w
		}
		return size
	}

	l := v.list
	if highlight {
		for i, name := range sty.stations {
			y := float64(l.Min.Y) + float64(i)*v.rowH
			cv.SetFillStyle(white)
			cv.FillRect(float64(l.Min.X), y+v.rowH*0.1, float64(l.Dx()), v.rowH*0.8)
			text(name, float64(l.Min.X)+v.fs*0.6, y+v.rowH/2, bold, v.fs, 0, color.NRGBA{0, 0, 0, 255})
		}
		img := be.Image
		return crop(img, l)
	}

	ty := H * 0.085
	x0, x1 := W*0.05, W*0.95
	if sty.subtabs == nil {
		step := (x1 - x0) / float64(len(sty.tabs))
		base := ty + v.fs*0.9
		var ax0, ax1 float64
		for i, name := range sty.tabs {
			cx := x0 + step*(float64(i)+0.5)
			tw := text(name, cx, ty, bold, v.fs*1.05, 0.5, white)
			if i == sty.tab {
				ax0, ax1 = cx-tw/2-v.fs*0.5, cx+tw/2+v.fs*0.5
			}
		}
		line(white, x0, base, ax0, base, ax0, ty-v.fs*0.9, ax1, ty-v.fs*0.9, ax1, base, x1, base)
		line(white, x0, base, x0, base+v.fs*0.6)
		line(white, x1, base, x1, base+v.fs*0.6)
	} else {
		tw := text(sty.tabs[sty.tab], x0+v.fs*0.6, ty, bold, v.fs*1.2, 0, white)
		stats := "HP 205/230   AP 75/80   XP 1420/2150"
		text(stats, x1, ty, mono, fit(stats, mono, v.fs*0.8, x1-x0-tw-v.fs*2), 1, white)
		base := ty + v.fs*0.95
		line(white, x0, base+v.fs*0.6, x0, base, x1, base, x1, base+v.fs*0.6)
		by := H * 0.93
		step := (x1 - x0) / float64(len(sty.subtabs))
		line(white, x0, by-v.fs*0.9, x0, by-v.fs*0.3)
		line(white, x1, by-v.fs*0.9, x1, by-v.fs*0.3)
		line(white, x0, by-v.fs*0.6, x1, by-v.fs*0.6)
		size := v.fs * 0.8
		for _, name := range sty.subtabs {
			size = math.Min(size, fit(name, mono, v.fs*0.8, step*0.86))
		}
		for i, name := range sty.subtabs {
			cx := x0 + step*(float64(i)+0.5)
			c := dim
			if i == sty.subtab {
				c = white
			}
			tw := text(name, cx, by+v.fs*0.2, mono, size, 0.5, c)
			if i == sty.subtab {
				line(white, cx-tw/2-v.fs*0.4, by-v.fs*0.6, cx-tw/2-v.fs*0.4, by+v.fs*0.9, cx+tw/2+v.fs*0.4, by+v.fs*0.9, cx+tw/2+v.fs*0.4, by-v.fs*0.6)
			}
		}
	}

	for i, name := range sty.stations {
		y := float64(l.Min.Y) + float64(i)*v.rowH
		text(name, float64(l.Min.X)+v.fs*0.6, y+v.rowH/2, mono, v.fs, 0, white)
	}

	b := v.box
	bx0, by0, bx1, by1 := float64(b.Min.X), float64(b.Min.Y), float64(b.Max.X), float64(b.Max.Y)
	line(white, bx0, by1, bx1, by1, bx1, by0)
	for i := 0; i <= 50; i++ {
		t := float64(i) / 50
		k := v.fs * 0.25
		if i%5 == 0 {
			k = v.fs * 0.55
		}
		x := bx0 + (bx1-bx0)*t
		line(white, x, by1, x, by1-k)
		y := by0 + (by1-by0)*t
		line(white, bx1, y, bx1-k, y)
	}
	cv.SetFillStyle(color.NRGBA{255, 255, 255, 22})
	for i := 1; i < 10; i++ {
		x := bx0 + (bx1-bx0)*float64(i)/10
		cv.FillRect(x, by0, math.Max(1, u), by1-by0)
		y := by0 + (by1-by0)*float64(i)/10
		cv.FillRect(bx0, y, bx1-bx0, math.Max(1, u))
	}

	if sty.footer != "" {
		fy := H * 0.935
		cv.SetFillStyle(color.NRGBA{255, 255, 255, 60})
		cv.FillRect(x0, fy-v.fs*0.8, x1-x0, v.fs*1.6)
		text(sty.footer, W/2, fy, mono, v.fs*0.85, 0.5, white)
	}
	if sty.status {
		fy := H * 0.935
		bw := (x1 - x0) * 0.26
		text("HP", x0, fy, bold, v.fs*0.9, 0, white)
		cv.SetFillStyle(white)
		cv.FillRect(x0+v.fs*1.8, fy-v.fs*0.3, bw*0.82, v.fs*0.6)
		text("LVL 76", W/2, fy, bold, v.fs*0.9, 0.5, white)
		text("AP", x1, fy, bold, v.fs*0.9, 1, white)
		cv.FillRect(x1-v.fs*1.8-bw*0.7, fy-v.fs*0.3, bw*0.7, v.fs*0.6)
		cv.SetFillStyle(color.NRGBA{255, 255, 255, 70})
		cv.FillRect(x1-v.fs*1.8-bw, fy-v.fs*0.3, bw*0.3, v.fs*0.6)
		cv.FillRect(x0+v.fs*1.8+bw*0.82, fy-v.fs*0.3, bw*0.18, v.fs*0.6)
	}
	return be.Image
}

func (v *pipboy) advance(f frame, x Input) {
	dt, t := f.dt, f.t
	want := 2
	switch f.state {
	case listening:
		want = 0
		v.heard = t
	case responding:
		want = 1
		if t-v.heard < pipMicHold {
			want = 0
		}
	}
	if want != v.active {
		v.active, v.since = want, t
	}
	switch want {
	case 0:
		f.wave = waveOf(x.Mic)
	case 1:
		f.wave = waveOf(x.Speaker)
	}
	talk := want < 2
	top := 0.0
	for _, w := range f.wave {
		top = math.Max(top, math.Abs(w))
	}
	v.peak = math.Max(pipFloor, follow(v.peak, top, 0.9, 0.05, dt))
	k := 1 - math.Pow(0.002, dt)
	for i := range v.wave {
		x := float64(i) / (pipSamples - 1)
		var target float64
		if talk {
			wi := int(x * float64(len(f.wave)-1))
			target = 0.5 - 0.44*math.Tanh(1.3*f.wave[wi]/v.peak)
		} else {
			target = 0.5 - 0.2*math.Sin(2*math.Pi*x*2.5-t*2.4)*(0.7+0.3*math.Sin(t*0.6))
		}
		v.wave[i] += (math.Max(0.03, math.Min(0.97, target)) - v.wave[i]) * k
	}
}

func waveOf(a analysis.Analysis) (w [analysis.WavePoints]float64) {
	for i := range w {
		w[i] = float64(a.Wave[i])
	}
	return w
}

//go:embed pipboy.glsl
var pipboyShader string

func (v *pipboy) Shade(g GL, fresh bool, in ui.Rect, x Input) error {
	if !v.st.measure(in) {
		return nil
	}
	if fresh {
		if err := g.Program(pipboyShader, Light); err != nil {
			return err
		}
	}
	if fresh || v.w != v.st.w || v.h != v.st.h {
		v.layout()
		var base, lit *image.RGBA
		var wg sync.WaitGroup
		wg.Go(func() { lit = v.paint(true) })
		base = v.paint(false)
		wg.Wait()
		art := image.NewRGBA(image.Rect(0, 0, v.st.w, v.st.h+lit.Bounds().Dy()))
		copy(art.Pix, base.Pix)
		for y := range lit.Bounds().Dy() {
			copy(art.Pix[art.PixOffset(0, v.st.h+y):], lit.Pix[y*lit.Stride:(y+1)*lit.Stride])
		}
		if err := g.Texture(0, art); err != nil {
			return err
		}
		for i := range v.wave {
			v.wave[i] = 0.5
		}
	}
	f := v.fr.next(x)
	v.advance(f, x)
	wave := image.NewRGBA(image.Rect(0, 0, pipSamples, 1))
	b := v.box
	for i, s := range v.wave {
		prev, next := v.wave[max(i-1, 0)], v.wave[min(i+1, pipSamples-1)]
		slope := (next - prev) * float64(b.Dy()) / (float64(min(i+1, pipSamples-1)-max(i-1, 0)) / (pipSamples - 1) * float64(b.Dx()))
		q, r := uint32(clamp01(s)*65535), uint32(clamp01(slope/pipSlope+0.5)*65535)
		wave.Pix[i*4], wave.Pix[i*4+1], wave.Pix[i*4+2], wave.Pix[i*4+3] = uint8(q>>8), uint8(q), uint8(r>>8), uint8(r)
	}
	if err := g.Texture(1, wave); err != nil {
		return err
	}
	sty := pipStyles[v.kind]
	vals := make([]float32, 26)
	vals[0], vals[1], vals[2], vals[3], vals[4], vals[5] = float32(v.st.w), float32(v.st.h), float32(sty.warp), float32(sty.scan*v.st.u), float32(sty.scanDeep), float32(f.t)
	vals[6], vals[7], vals[8] = float32(sty.tint[0]), float32(sty.tint[1]), float32(sty.tint[2])
	vals[9] = float32(0.97 + 0.03*math.Sin(f.t*53) + 0.02*math.Sin(f.t*7.3))
	l := v.list
	vals[10], vals[11], vals[12], vals[13] = float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy())
	vals[15] = float32(math.Max(1.5, 2.6*v.st.u))
	vals[16], vals[17], vals[18], vals[19] = float32(l.Min.X), float32(l.Min.Y), float32(l.Dx()), float32(l.Dy())
	vals[20], vals[21] = float32(v.rowH), float32(v.active)
	vals[22], vals[23] = float32(v.st.u), float32(sty.roll)
	vals[24], vals[25] = float32(v.st.h+l.Dy()), pipSlope
	return g.Values(vals, float32(0.5+0.35*f.level), 0.012, 2)
}
