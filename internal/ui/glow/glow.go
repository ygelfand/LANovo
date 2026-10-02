package glow

import (
	"image"
	"image/color"
	"math"

	"github.com/ygelfand/LANovo/internal/ui"
)

type Stop struct {
	At float32
	C  color.NRGBA
}

func Clear(img *image.RGBA, c color.NRGBA) {
	b := img.Bounds()
	if b.Empty() {
		return
	}
	row := img.Pix[:b.Dx()*4]
	for i := 0; i < len(row); i += 4 {
		row[i], row[i+1], row[i+2], row[i+3] = c.R, c.G, c.B, 255
	}
	for y := 1; y < b.Dy(); y++ {
		copy(img.Pix[y*img.Stride:y*img.Stride+len(row)], row)
	}
}

func Radial(img *image.RGBA, cx, cy, r0, r1 float32, stops []Stop, additive bool) {
	b := img.Bounds()
	span := r1 - r0
	if span <= 0 || len(stops) == 0 {
		return
	}
	x0, x1 := max(int(cx-r1), 0), min(int(cx+r1)+1, b.Dx())
	y0, y1 := max(int(cy-r1), 0), min(int(cy+r1)+1, b.Dy())

	n := min(squares, max(int(r1*r1), 16))
	sq := make([]color.NRGBA, n)
	for k := range sq {
		d := float32(math.Sqrt(float64(k)/float64(n-1))) * r1
		sq[k] = sample(stops, (d-r0)/span)
	}
	scale := float32(n-1) / (r1 * r1)
	edge := max(r1-1, 0) * max(r1-1, 0)
	if additive {
		radialAdd(img, cx, cy, r1, sq, scale, edge, y0, y1, x0, x1)
		return
	}

	Rows(y0, y1, x1-x0, func(ya, yb int) {
		for y := ya; y < yb; y++ {
			dy := float32(y) + 0.5 - cy
			dy2 := dy * dy
			row := img.Pix[y*img.Stride:]
			dx := float32(x0) + 0.5 - cx
			for x := x0; x < x1; x++ {
				d2 := dx*dx + dy2
				dx++
				if d2 >= r1*r1 {
					continue
				}
				c := sq[min(int(d2*scale), n-1)]
				if d2 > edge {
					c.A = uint8(float32(c.A) * min(r1-float32(math.Sqrt(float64(d2))), 1))
				}
				put(row[x*4:x*4+3:x*4+3], c, additive)
			}
		}
	})
}

const squares = 4096

func radialAdd(img *image.RGBA, cx, cy, r1 float32, sq []color.NRGBA, scale, edge float32, y0, y1, x0, x1 int) {
	n := len(sq)
	pre := make([][3]uint8, n)
	last := -1
	for k, c := range sq {
		a := uint32(c.A)
		pre[k] = [3]uint8{uint8(div255(uint32(c.R) * a)), uint8(div255(uint32(c.G) * a)), uint8(div255(uint32(c.B) * a))}
		if c.A > 0 {
			last = k
		}
	}
	if last < 0 {
		return
	}
	reach := min(r1*r1, float32(last+2)/scale)
	Rows(y0, y1, x1-x0, func(ya, yb int) {
		for y := ya; y < yb; y++ {
			dy := float32(y) + 0.5 - cy
			dy2 := dy * dy
			if dy2 >= reach {
				continue
			}
			half := float32(math.Sqrt(float64(reach - dy2)))
			xa, xb := max(int(cx-half)-1, x0), min(int(cx+half)+2, x1)
			row := img.Pix[y*img.Stride:]
			dx := float32(xa) + 0.5 - cx
			for x := xa; x < xb; x++ {
				d2 := dx*dx + dy2
				dx++
				if d2 >= r1*r1 {
					continue
				}
				k := min(int(d2*scale), n-1)
				p := pre[k]
				if d2 > edge {
					c := sq[k]
					c.A = uint8(float32(c.A) * min(r1-float32(math.Sqrt(float64(d2))), 1))
					a := uint32(c.A)
					p = [3]uint8{uint8(div255(uint32(c.R) * a)), uint8(div255(uint32(c.G) * a)), uint8(div255(uint32(c.B) * a))}
				}
				d := row[x*4 : x*4+3 : x*4+3]
				d[0] = sat8(uint32(d[0]) + uint32(p[0]))
				d[1] = sat8(uint32(d[1]) + uint32(p[1]))
				d[2] = sat8(uint32(d[2]) + uint32(p[2]))
			}
		}
	})
}

// RadialSquashed is Radial with the vertical axis scaled by sy, an ellipse sy times as tall as wide.
func RadialSquashed(img *image.RGBA, cx, cy, r, sy float32, stops []Stop, additive bool) {
	b := img.Bounds()
	if r <= 0 || sy <= 0 || len(stops) == 0 {
		return
	}
	lut := ramp(stops)
	x0, x1 := max(int(cx-r), 0), min(int(cx+r)+1, b.Dx())
	y0, y1 := max(int(cy-r*sy), 0), min(int(cy+r*sy)+1, b.Dy())
	inv := 1 / (r * r)
	Rows(y0, y1, x1-x0, func(ya, yb int) {
		for y := ya; y < yb; y++ {
			dy := (float32(y) + 0.5 - cy) / sy
			row := img.Pix[y*img.Stride:]
			for x := x0; x < x1; x++ {
				dx := float32(x) + 0.5 - cx
				q := (dx*dx + dy*dy) * inv
				if q >= 1 {
					continue
				}
				put(row[x*4:x*4+3:x*4+3], lut[clampi(int(float32(math.Sqrt(float64(q)))*255), 0, 255)], additive)
			}
		}
	})
}

func Linear(img *image.RGBA, x0, y0, x1, y1 float32, stops []Stop, additive bool) {
	b := img.Bounds()
	dx, dy := x1-x0, y1-y0
	n := dx*dx + dy*dy
	if n == 0 || len(stops) == 0 {
		return
	}
	lut := ramp(stops)
	Rows(0, b.Dy(), b.Dx(), func(ya, yb int) {
		for y := ya; y < yb; y++ {
			row := img.Pix[y*img.Stride:]
			for x := range b.Dx() {
				t := ((float32(x)+0.5-x0)*dx + (float32(y)+0.5-y0)*dy) / n
				put(row[x*4:x*4+3:x*4+3], lut[clampi(int(t*255), 0, 255)], additive)
			}
		}
	})
}

// LinearRect fills r with a gradient running from (x0, y0) to (x1, y1).
func LinearRect(img *image.RGBA, r image.Rectangle, x0, y0, x1, y1 float32, stops []Stop, additive bool) {
	r = r.Intersect(img.Bounds())
	dx, dy := x1-x0, y1-y0
	n := dx*dx + dy*dy
	if n == 0 || len(stops) == 0 || r.Empty() {
		return
	}
	lut := ramp(stops)
	kx, ky := dx/n, dy/n
	Rows(r.Min.Y, r.Max.Y, r.Dx(), func(ya, yb int) {
		for y := ya; y < yb; y++ {
			row := img.Pix[y*img.Stride:]
			base := (float32(y)+0.5-y0)*ky - x0*kx
			for x := r.Min.X; x < r.Max.X; x++ {
				t := base + (float32(x)+0.5)*kx
				put(row[x*4:x*4+3:x*4+3], lut[clampi(int(t*255), 0, 255)], additive)
			}
		}
	})
}

// Ramp is a colour ramp sampled at 128 steps, bottom (0) to top (127).
type Ramp [128]color.NRGBA

func NewRamp(stops []Stop) Ramp {
	var r Ramp
	for i := range r {
		r[i] = sample(stops, float32(i)/127)
	}
	return r
}

// VStrip stretches the ramp over the column band [x, x+w) from top down h pixels, top of the ramp
// at the top, scaled by alpha.
func VStrip(img *image.RGBA, x, w int, top, h, alpha float32, r *Ramp, additive bool) {
	VBand(img, x, w, top, top+h, top, h, alpha, r, additive)
}

// VBand is VStrip painting only the rows from lo to hi.
func VBand(img *image.RGBA, x, w int, lo, hi, top, h, alpha float32, r *Ramp, additive bool) {
	b := img.Bounds()
	if h <= 0 {
		return
	}
	y0, y1 := max(int(lo), 0), min(int(hi), b.Dy())
	x0, x1 := max(x, 0), min(x+w, b.Dx())
	a := max(min(alpha, 1), 0)
	for y := y0; y < y1; y++ {
		k := clampi(int((top+h-float32(y))/h*127), 0, 127)
		c := r[k]
		c.A = uint8(float32(c.A) * a)
		if c.A == 0 {
			continue
		}
		fill(img.Pix[y*img.Stride:], x0, x1, c, additive)
	}
}

func fill(row []uint8, x0, x1 int, c color.NRGBA, additive bool) {
	if c.A == 0 || x1 <= x0 {
		return
	}
	row = row[x0*4 : x1*4]
	a := uint32(c.A)
	if additive {
		r, g, b := div255(uint32(c.R)*a), div255(uint32(c.G)*a), div255(uint32(c.B)*a)
		for i := 0; i+3 < len(row); i += 4 {
			d := row[i : i+3 : i+3]
			d[0] = sat8(uint32(d[0]) + r)
			d[1] = sat8(uint32(d[1]) + g)
			d[2] = sat8(uint32(d[2]) + b)
		}
		return
	}
	ia := 255 - a
	r, g, b := uint32(c.R)*a, uint32(c.G)*a, uint32(c.B)*a
	for i := 0; i+3 < len(row); i += 4 {
		d := row[i : i+3 : i+3]
		d[0] = uint8(div255(uint32(d[0])*ia + r))
		d[1] = uint8(div255(uint32(d[1])*ia + g))
		d[2] = uint8(div255(uint32(d[2])*ia + b))
	}
}

// Span blends c across row y from x0 up to x1.
func Span(img *image.RGBA, y, x0, x1 int, c color.NRGBA, additive bool) {
	b := img.Bounds()
	if y < 0 || y >= b.Dy() {
		return
	}
	fill(img.Pix[y*img.Stride:], max(x0, 0), min(x1, b.Dx()), c, additive)
}

func ramp(stops []Stop) [256]color.NRGBA {
	var lut [256]color.NRGBA
	for i := range lut {
		t := float32(i) / 255
		lut[i] = sample(stops, t)
	}
	return lut
}

func sample(stops []Stop, t float32) color.NRGBA {
	if t <= stops[0].At {
		return stops[0].C
	}
	for i := 1; i < len(stops); i++ {
		if t <= stops[i].At {
			a, b := stops[i-1], stops[i]
			k := float32(0)
			if b.At > a.At {
				k = (t - a.At) / (b.At - a.At)
			}
			mix := func(p, q uint8) uint8 { return uint8(float32(p) + (float32(q)-float32(p))*k + 0.5) }
			return color.NRGBA{mix(a.C.R, b.C.R), mix(a.C.G, b.C.G), mix(a.C.B, b.C.B), mix(a.C.A, b.C.A)}
		}
	}
	return stops[len(stops)-1].C
}

func put(d []uint8, c color.NRGBA, additive bool) {
	if c.A == 0 {
		return
	}
	a := uint32(c.A)
	if additive {
		d[0] = sat8(uint32(d[0]) + div255(uint32(c.R)*a))
		d[1] = sat8(uint32(d[1]) + div255(uint32(c.G)*a))
		d[2] = sat8(uint32(d[2]) + div255(uint32(c.B)*a))
		return
	}
	d[0] = uint8(div255(uint32(d[0])*(255-a) + uint32(c.R)*a))
	d[1] = uint8(div255(uint32(d[1])*(255-a) + uint32(c.G)*a))
	d[2] = uint8(div255(uint32(d[2])*(255-a) + uint32(c.B)*a))
}

// div255 is x/255 for x up to 255*255, without a division: GOARM=7 has none in hardware.
func div255(x uint32) uint32 { return (x + 1 + (x >> 8)) >> 8 }

// Add lays src onto dst additively; the reference's "lighter" composite.
func Add(dst, src *image.RGBA, amount float32) {
	b := dst.Bounds().Intersect(src.Bounds())
	k := uint32(max(amount, 0) * 256)
	for y := range b.Dy() {
		d := dst.Pix[y*dst.Stride : y*dst.Stride+b.Dx()*4]
		s := src.Pix[y*src.Stride : y*src.Stride+b.Dx()*4]
		for i := 0; i < len(d); i += 4 {
			d[i] = sat8(uint32(d[i]) + uint32(s[i])*k>>8)
			d[i+1] = sat8(uint32(d[i+1]) + uint32(s[i+1])*k>>8)
			d[i+2] = sat8(uint32(d[i+2]) + uint32(s[i+2])*k>>8)
		}
	}
}

func sat8(v uint32) uint8 {
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func clampi(v, lo, hi int) int { return max(lo, min(v, hi)) }

// Blit copies img into box on s, each pixel scale times across and down.
func Blit(s ui.Surface, img *image.RGBA, box ui.Rect, scale int) {
	ui.DrawRGBA(s, box.X, box.Y, img, scale, box)
}
