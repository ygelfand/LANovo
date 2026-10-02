package visual

import (
	"image"
	"image/color"
	"math"

	"github.com/tfriedel6/canvas"
	fx "github.com/ygelfand/LANovo/internal/ui/glow"
)

func crop(img *image.RGBA, r image.Rectangle) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := range r.Dy() {
		copy(out.Pix[y*out.Stride:(y+1)*out.Stride], img.Pix[img.PixOffset(r.Min.X, r.Min.Y+y):])
	}
	return out
}

func fillRect(img *image.RGBA, x, y, w, h float64, c color.NRGBA) {
	for py := int(math.Round(y)); py < int(math.Round(y+h)); py++ {
		fx.Span(img, py, int(math.Round(x)), int(math.Round(x+w)), c, false)
	}
}

func roundRect(c *canvas.Canvas, x, y, w, h, r float64) {
	c.BeginPath()
	c.MoveTo(x+r, y)
	c.LineTo(x+w-r, y)
	c.QuadraticCurveTo(x+w, y, x+w, y+r)
	c.LineTo(x+w, y+h-r)
	c.QuadraticCurveTo(x+w, y+h, x+w-r, y+h)
	c.LineTo(x+r, y+h)
	c.QuadraticCurveTo(x, y+h, x, y+h-r)
	c.LineTo(x, y+r)
	c.QuadraticCurveTo(x, y, x+r, y)
	c.ClosePath()
}

func rounded(img *image.RGBA, x, y, w, h, r float64, draw func(sub *image.RGBA, ox, oy float64)) {
	rect := image.Rect(int(x), int(y), int(math.Ceil(x+w)), int(math.Ceil(y+h))).Intersect(img.Bounds())
	if rect.Empty() {
		return
	}
	saved := make([]uint8, rect.Dx()*rect.Dy()*4)
	for j := range rect.Dy() {
		copy(saved[j*rect.Dx()*4:(j+1)*rect.Dx()*4], img.Pix[(rect.Min.Y+j)*img.Stride+rect.Min.X*4:])
	}
	draw(img.SubImage(rect).(*image.RGBA), float64(rect.Min.X), float64(rect.Min.Y))
	edge := int(math.Ceil(r)) + 2
	for j := range rect.Dy() {
		py := float64(rect.Min.Y+j) + 0.5
		band := py-y < float64(edge) || y+h-py < float64(edge)
		for i := 0; i < rect.Dx(); i++ {
			px := float64(rect.Min.X+i) + 0.5
			if !band && px-x >= float64(edge) && x+w-px >= float64(edge) {
				i = max(i, int(math.Floor(x+w-float64(edge)-0.5))-rect.Min.X-1)
				continue
			}
			k := roundCover(px, py, x, y, w, h, r)
			if k >= 1 {
				continue
			}
			d := img.Pix[(rect.Min.Y+j)*img.Stride+(rect.Min.X+i)*4:]
			s := saved[(j*rect.Dx()+i)*4:]
			for c := range 3 {
				d[c] = uint8(float64(s[c])*(1-k) + float64(d[c])*k)
			}
		}
	}
}

func roundCover(px, py, x, y, w, h, r float64) float64 {
	dx := max(x+r-px, px-(x+w-r), 0)
	dy := max(y+r-py, py-(y+h-r), 0)
	if dx == 0 {
		return clamp01(r - dy + 0.5)
	}
	if dy == 0 {
		return clamp01(r - dx + 0.5)
	}
	return clamp01(r - math.Hypot(dx, dy) + 0.5)
}

func gradAt(stops []fx.Stop, t float64) color.NRGBA {
	t = clamp01(t)
	for i := 1; i < len(stops); i++ {
		if float32(t) <= stops[i].At {
			a, b := stops[i-1], stops[i]
			k := (float32(t) - a.At) / (b.At - a.At)
			m := func(p, q uint8) uint8 { return uint8(float32(p) + (float32(q)-float32(p))*k) }
			return color.NRGBA{m(a.C.R, b.C.R), m(a.C.G, b.C.G), m(a.C.B, b.C.B), m(a.C.A, b.C.A)}
		}
	}
	return stops[len(stops)-1].C
}

func hex(v uint32, a float64) color.NRGBA {
	return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), uint8(clamp01(a) * 255)}
}

// shadow darkens around a rounded rectangle the way a canvas shadowBlur of blur does.
func shadow(img *image.RGBA, x, y, w, h, r, blur, a float64) {
	b := img.Bounds()
	cx, cy, hw, hh := x+w/2, y+h/2, w/2-r, h/2-r
	fx.Rows(max(int(y-blur), 0), min(int(y+h+blur)+1, b.Dy()), int(w+2*blur), func(ya, yb int) {
		for py := ya; py < yb; py++ {
			qy := math.Abs(float64(py)+0.5-cy) - hh
			row := img.Pix[py*img.Stride:]
			for px := max(int(x-blur), 0); px < min(int(x+w+blur)+1, b.Dx()); px++ {
				qx := math.Abs(float64(px)+0.5-cx) - hw
				var d float64
				switch {
				case qx <= 0 && qy <= 0:
					d = max(qx, qy) - r
				case qx <= 0:
					d = qy - r
				case qy <= 0:
					d = qx - r
				default:
					d = math.Hypot(qx, qy) - r
				}
				k := a * (1 - smoothstep(-blur, blur, d))
				if k <= 0 {
					continue
				}
				p := row[px*4 : px*4+3 : px*4+3]
				p[0], p[1], p[2] = uint8(float64(p[0])*(1-k)), uint8(float64(p[1])*(1-k)), uint8(float64(p[2])*(1-k))
			}
		}
	})
}

func nrgba(c [3]float64, a float64) color.NRGBA {
	return color.NRGBA{uint8(clamp01(c[0]/255) * 255), uint8(clamp01(c[1]/255) * 255), uint8(clamp01(c[2]/255) * 255), uint8(clamp01(a) * 255)}
}

func mixc(a, b [3]float64, t float64) [3]float64 {
	return [3]float64{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

func brickAt(fx, fy, s float64) (r, g, b, tone float64) {
	bw, bh := s*0.12, s*0.05
	row := math.Floor(fy / bh)
	off := math.Mod(row, 2) * bw / 2
	col := math.Floor((fx + off) / bw)
	tone = 0.5 + 0.5*cellHash(int(col), int(row), 7)
	if math.Mod(fx+off, bw) < s*0.006 || math.Mod(fy, bh) < s*0.006 {
		return 0.16, 0.14, 0.12, tone
	}
	grain := 0.9 + 0.1*math.Sin(fx*1.7+fy*2.3)
	return (0.3 + 0.14*tone) * grain, (0.13 + 0.05*tone) * grain, (0.09 + 0.03*tone) * grain, tone
}
