package glow

import (
	"image"
	"image/color"
	"math"
)

// Square fills an axis-aligned square of side size centred on x, y, blending c by its alpha and by
// how much of each edge pixel it covers.
func Square(img *image.RGBA, x, y, size float32, c color.NRGBA, additive bool) {
	b := img.Bounds()
	x0, y0, x1, y1 := x-size/2, y-size/2, x+size/2, y+size/2
	for py := max(int(y0), 0); py < min(int(math.Ceil(float64(y1))), b.Dy()); py++ {
		cy := cover(float32(py), y0, y1)
		row := img.Pix[py*img.Stride:]
		for px := max(int(x0), 0); px < min(int(math.Ceil(float64(x1))), b.Dx()); px++ {
			k := cover(float32(px), x0, x1) * cy
			if k <= 0 {
				continue
			}
			blend(row[px*4:px*4+3:px*4+3], c, k, additive)
		}
	}
}

func cover(p, lo, hi float32) float32 {
	return max(0, min(p+1, hi)-max(p, lo))
}

// Line draws a one pixel antialiased line (Xiaolin Wu).
func Line(img *image.RGBA, x0, y0, x1, y1 float32, c color.NRGBA, additive bool) {
	line(img, x0, y0, x1, y1, c, additive, false)
}

func line(img *image.RGBA, x0, y0, x1, y1 float32, c color.NRGBA, additive, joined bool) {
	steep := abs(y1-y0) > abs(x1-x0)
	if steep {
		x0, y0, x1, y1 = y0, x0, y1, x1
	}
	skipLo, skipHi := joined, false
	if x0 > x1 {
		x0, x1, y0, y1 = x1, x0, y1, y0
		skipLo, skipHi = false, joined
	}
	dx := x1 - x0
	grad := float32(1)
	if dx > 0 {
		grad = (y1 - y0) / dx
	}
	b := img.Bounds()
	plot := func(x, y int, k float32) {
		if steep {
			x, y = y, x
		}
		if x < 0 || y < 0 || x >= b.Dx() || y >= b.Dy() || k <= 0 {
			return
		}
		i := y*img.Stride + x*4
		blend(img.Pix[i:i+3:i+3], c, k, additive)
	}
	y := y0 + grad*(float32(int(x0))-x0)
	for x := int(x0); x <= int(x1); x++ {
		iy := int(math.Floor(float64(y)))
		f := y - float32(iy)
		y += grad
		if (skipLo && x == int(x0)) || (skipHi && x == int(x1)) {
			continue
		}
		plot(x, iy, 1-f)
		plot(x, iy+1, f)
	}
}

// Polyline strokes the points (x0, y0, x1, y1, ...) at width with round ends. Under a pixel wide it
// is a hairline with its alpha scaled by the width.
func Polyline(img *image.RGBA, pts []float32, width float32, c color.NRGBA, additive bool) {
	if len(pts) < 4 {
		return
	}
	if width <= 1.2 {
		k := min(width, 1)
		c.A = uint8(float32(c.A) * k)
		for i := 2; i+1 < len(pts); i += 2 {
			line(img, pts[i-2], pts[i-1], pts[i], pts[i+1], c, additive, i > 2)
		}
		return
	}
	b := img.Bounds()
	hw := width / 2
	for i := 2; i+1 < len(pts); i += 2 {
		ax, ay, bx, by := pts[i-2], pts[i-1], pts[i], pts[i+1]
		dx, dy := bx-ax, by-ay
		ll := dx*dx + dy*dy
		inv := float32(0)
		if ll > 0 {
			inv = 1 / ll
		}
		outer, inner := (hw+0.5)*(hw+0.5), max(hw-0.5, 0)*max(hw-0.5, 0)
		x0, x1 := max(int(min(ax, bx)-hw-1), 0), min(int(max(ax, bx)+hw+2), b.Dx())
		y0, y1 := max(int(min(ay, by)-hw-1), 0), min(int(max(ay, by)+hw+2), b.Dy())
		last := i+2 >= len(pts)
		for y := y0; y < y1; y++ {
			py := float32(y) + 0.5
			row := img.Pix[y*img.Stride:]
			for x := x0; x < x1; x++ {
				px := float32(x) + 0.5
				t := min(max(((px-ax)*dx+(py-ay)*dy)*inv, 0), 1)
				if t >= 1 && !last {
					continue
				}
				ex, ey := px-(ax+dx*t), py-(ay+dy*t)
				d2 := ex*ex + ey*ey
				if d2 >= outer {
					continue
				}
				k := float32(1)
				if d2 > inner {
					k = min(hw-float32(math.Sqrt(float64(d2)))+0.5, 1)
				}
				blend(row[x*4:x*4+3:x*4+3], c, k, additive)
			}
		}
	}
}

// Under fills between a curve (x ascending: x0, y0, x1, y1, ...) and the row base, column by column.
func Under(img *image.RGBA, pts []float32, base float32, c color.NRGBA) {
	b := img.Bounds()
	yb := min(int(base), b.Dy()-1)
	for i := 2; i+1 < len(pts); i += 2 {
		ax, ay, bx, by := pts[i-2], pts[i-1], pts[i], pts[i+1]
		if bx <= ax {
			continue
		}
		slope := (by - ay) / (bx - ax)
		for x := max(int(ax), 0); x < min(int(bx)+1, b.Dx()); x++ {
			y := ay + (float32(x)+0.5-ax)*slope
			for py := max(int(y)+1, 0); py <= yb; py++ {
				i := py*img.Stride + x*4
				img.Pix[i], img.Pix[i+1], img.Pix[i+2] = c.R, c.G, c.B
			}
		}
	}
}

// Over lays a premultiplied RGBA layer onto dst within r, the canvas's source-over.
func Over(dst, src *image.RGBA, r image.Rectangle) {
	r = r.Intersect(dst.Bounds()).Intersect(src.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		d := dst.Pix[y*dst.Stride+r.Min.X*4 : y*dst.Stride+r.Max.X*4]
		s := src.Pix[y*src.Stride+r.Min.X*4 : y*src.Stride+r.Max.X*4]
		for i := 0; i < len(d); i += 4 {
			a := uint32(s[i+3])
			if a == 0 {
				continue
			}
			inv := 255 - a
			d[i] = sat8(uint32(s[i]) + div255(uint32(d[i])*inv))
			d[i+1] = sat8(uint32(s[i+1]) + div255(uint32(d[i+1])*inv))
			d[i+2] = sat8(uint32(s[i+2]) + div255(uint32(d[i+2])*inv))
		}
	}
}

// Wedge fills a round-capped segment whose half-width runs from w0 at (ax, ay) to w1 at (bx, by),
// its edge feathered over soft pixels.
func Wedge(img *image.RGBA, ax, ay, bx, by, w0, w1, soft float32, c color.NRGBA, additive bool) {
	b := img.Bounds()
	dx, dy := bx-ax, by-ay
	ll := dx*dx + dy*dy
	if ll == 0 {
		return
	}
	inv, ll2 := 1/ll, float32(math.Sqrt(float64(ll)))
	soft = max(soft, 1)
	reach := max(w0, w1) + soft
	x0, x1 := max(int(min(ax, bx)-reach), 0), min(int(max(ax, bx)+reach)+1, b.Dx())
	y0, y1 := max(int(min(ay, by)-reach), 0), min(int(max(ay, by)+reach)+1, b.Dy())
	nx, ny := -dy/ll2, dx/ll2
	for y := y0; y < y1; y++ {
		py := float32(y) + 0.5
		row := img.Pix[y*img.Stride:]
		for x := x0; x < x1; x++ {
			px := float32(x) + 0.5
			t := ((px-ax)*dx + (py-ay)*dy) * inv
			var d float32
			if t < 0 || t > 1 {
				t = min(max(t, 0), 1)
				ex, ey := px-ax-dx*t, py-ay-dy*t
				d = float32(math.Sqrt(float64(ex*ex + ey*ey)))
			} else {
				d = abs((px-ax)*nx + (py-ay)*ny)
			}
			k := (w0+(w1-w0)*t-d)/soft + 0.5
			if k <= 0 {
				continue
			}
			blend(row[x*4:x*4+3:x*4+3], c, min(k, 1), additive)
		}
	}
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func blend(d []uint8, c color.NRGBA, k float32, additive bool) {
	a := float32(c.A) / 255 * k
	if additive {
		d[0] = sat8(uint32(float32(d[0]) + float32(c.R)*a))
		d[1] = sat8(uint32(float32(d[1]) + float32(c.G)*a))
		d[2] = sat8(uint32(float32(d[2]) + float32(c.B)*a))
		return
	}
	d[0] = uint8(float32(d[0])*(1-a) + float32(c.R)*a)
	d[1] = uint8(float32(d[1])*(1-a) + float32(c.G)*a)
	d[2] = uint8(float32(d[2])*(1-a) + float32(c.B)*a)
}
