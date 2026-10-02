package glow

import (
	"image"
	"image/color"
	"math"
)

// Feedback redraws src into dst zoomed by scale and turned by rot about (cx, cy), faded by keep.
func Feedback(dst, src *image.RGBA, cx, cy, scale, rot, keep float32) {
	b := dst.Bounds()
	W, H := b.Dx(), b.Dy()
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	c, s := float32(math.Cos(float64(rot))), float32(math.Sin(float64(rot)))
	inv := 1 / scale
	k := uint32(max(min(keep, 1), 0) * 256)
	for y := range H {
		dy := float32(y) + 0.5 - cy
		row := dst.Pix[y*dst.Stride : y*dst.Stride+W*4]
		dx := 0.5 - cx
		ux, uy := (c*dx+s*dy)*inv+cx, (-s*dx+c*dy)*inv+cy
		stepX, stepY := c*inv, -s*inv
		for x := 0; x < W; x++ {
			sx, sy := int(ux), int(uy)
			ux += stepX
			uy += stepY
			i := x * 4
			if sx < 0 || sy < 0 || sx >= sw || sy >= sh {
				row[i], row[i+1], row[i+2] = 0, 0, 0
				continue
			}
			j := sy*src.Stride + sx*4
			row[i] = uint8(uint32(src.Pix[j]) * k >> 8)
			row[i+1] = uint8(uint32(src.Pix[j+1]) * k >> 8)
			row[i+2] = uint8(uint32(src.Pix[j+2]) * k >> 8)
		}
	}
}

// Fade scales every pixel by keep, the canvas's translucent black fill over the last frame.
func Fade(img *image.RGBA, keep float32) {
	k := uint32(max(min(keep, 1), 0) * 256)
	p := img.Pix
	for i := 0; i+3 < len(p); i += 4 {
		p[i] = uint8(uint32(p[i]) * k >> 8)
		p[i+1] = uint8(uint32(p[i+1]) * k >> 8)
		p[i+2] = uint8(uint32(p[i+2]) * k >> 8)
	}
}

// AddScaled adds src, scaled up to dst's size, onto dst.
func AddScaled(dst, src *image.RGBA) {
	b, sb := dst.Bounds(), src.Bounds()
	W, H, sw, sh := b.Dx(), b.Dy(), sb.Dx(), sb.Dy()
	cols := make([]int32, W)
	for x := range W {
		cols[x] = int32(min(x*sw/W, sw-1) * 4)
	}
	fy, stepY := 0, sh
	sy := 0
	for y := range H {
		row := dst.Pix[y*dst.Stride : y*dst.Stride+W*4]
		srow := src.Pix[sy*src.Stride:]
		for fy += stepY; fy >= H && sy < sh-1; fy -= H {
			sy++
		}
		for x := range W {
			i, j := x*4, cols[x]
			row[i] = sat8(uint32(row[i]) + uint32(srow[j]))
			row[i+1] = sat8(uint32(row[i+1]) + uint32(srow[j+1]))
			row[i+2] = sat8(uint32(row[i+2]) + uint32(srow[j+2]))
		}
	}
}

// Ring strokes a circle.
func Ring(img *image.RGBA, cx, cy, r, width float32, c color.NRGBA, additive bool) {
	n := max(int(r*0.8), 24)
	pts := make([]float32, 0, (n+1)*2)
	for i := 0; i <= n; i++ {
		a := float64(i) / float64(n) * 2 * math.Pi
		pts = append(pts, cx+r*float32(math.Cos(a)), cy+r*float32(math.Sin(a)))
	}
	Polyline(img, pts, width, c, additive)
}
