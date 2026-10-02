package poster

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"

	_ "golang.org/x/image/webp"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

const (
	scrimStrength = 0.62
	scrimPad      = 0.08
	scrimSoft     = 0.18
)

func decode(data []byte) (image.Image, error) {
	img, kind, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("poster: decoding %d bytes: %w", len(data), err)
	}
	if b := img.Bounds(); b.Dx() < 2 || b.Dy() < 2 {
		return nil, fmt.Errorf("poster: a %dx%d %s", b.Dx(), b.Dy(), kind)
	}
	return img, nil
}

func cover(src image.Rectangle, w, h int) image.Rectangle {
	sw, sh := src.Dx(), src.Dy()
	if sw*h > sh*w {
		cw := sh * w / h
		x := src.Min.X + (sw-cw)/2
		return image.Rect(x, src.Min.Y, x+cw, src.Max.Y)
	}
	ch := sw * h / w
	y := src.Min.Y + (sh-ch)/2
	return image.Rect(src.Min.X, y, src.Max.X, y+ch)
}

func compose(src image.Image, w, h int, box image.Rectangle, bg theme.Color) *image.RGBA {
	crop := cover(src.Bounds(), w, h)
	flat := image.NewRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	draw.Draw(flat, flat.Bounds(), src, crop.Min, draw.Src)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	bilinear(out, flat)
	scrim(out, box, bg)
	return out
}

func bilinear(dst, src *image.RGBA) {
	dw, dh := dst.Rect.Dx(), dst.Rect.Dy()
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	xs := make([]int32, dw)
	xw := make([]int32, dw)
	for x := range dw {
		f := max((float64(x)+0.5)*float64(sw)/float64(dw)-0.5, 0)
		i := min(int(f), sw-1)
		xs[x], xw[x] = int32(i), int32((f-float64(i))*256)
		if i == sw-1 {
			xw[x] = 0
		}
	}
	for y := range dh {
		f := max((float64(y)+0.5)*float64(sh)/float64(dh)-0.5, 0)
		y0 := min(int(f), sh-1)
		y1 := min(y0+1, sh-1)
		wy := int32((f - float64(y0)) * 256)
		r0 := src.Pix[y0*src.Stride : y0*src.Stride+sw*4]
		r1 := src.Pix[y1*src.Stride : y1*src.Stride+sw*4]
		out := dst.Pix[y*dst.Stride : y*dst.Stride+dw*4]
		for x := range dw {
			i, wx := int(xs[x])*4, xw[x]
			j := min(i+4, (sw-1)*4)
			o := out[x*4 : x*4+4 : x*4+4]
			a, b := r0[i:i+3:i+3], r0[j:j+3:j+3]
			c, d := r1[i:i+3:i+3], r1[j:j+3:j+3]
			ix, iy := 256-wx, 256-wy
			o[0] = uint8(((int32(a[0])*ix+int32(b[0])*wx)*iy + (int32(c[0])*ix+int32(d[0])*wx)*wy + 1<<15) >> 16)
			o[1] = uint8(((int32(a[1])*ix+int32(b[1])*wx)*iy + (int32(c[1])*ix+int32(d[1])*wx)*wy + 1<<15) >> 16)
			o[2] = uint8(((int32(a[2])*ix+int32(b[2])*wx)*iy + (int32(c[2])*ix+int32(d[2])*wx)*wy + 1<<15) >> 16)
			o[3] = 0xff
		}
	}
}

func scrim(img *image.RGBA, box image.Rectangle, bg theme.Color) {
	if box.Empty() {
		return
	}
	side := float64(min(box.Dx(), box.Dy()))
	pad, soft := side*scrimPad, side*scrimSoft
	r := pad + soft*0.5
	cx, cy := float64(box.Min.X+box.Max.X)/2, float64(box.Min.Y+box.Max.Y)/2
	hw, hh := float64(box.Dx())/2+pad-r, float64(box.Dy())/2+pad-r
	reach := box.Inset(-int(math.Ceil(pad + soft))).Intersect(img.Bounds())
	strength := scrimStrength
	full := int32(strength*256 + 0.5)
	inX0 := max(int(math.Ceil(cx-hw-0.5)), reach.Min.X)
	inX1 := min(int(math.Floor(cx+hw-0.5))+1, reach.Max.X)
	for y := reach.Min.Y; y < reach.Max.Y; y++ {
		qy := math.Abs(float64(y)+0.5-cy) - hh
		row := img.Pix[y*img.Stride:]
		for x := reach.Min.X; x < reach.Max.X; x++ {
			if qy <= 0 && x == inX0 && inX0 < inX1 {
				for i := inX0 * 4; i < inX1*4; i += 4 {
					p := row[i : i+3 : i+3]
					p[0] = mix(p[0], bg.R, full)
					p[1] = mix(p[1], bg.G, full)
					p[2] = mix(p[2], bg.B, full)
				}
				x = inX1 - 1
				continue
			}
			qx := math.Abs(float64(x)+0.5-cx) - hw
			k := full
			if qx > 0 || qy > 0 {
				d := math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - r
				if d > 0 {
					t := 1 - d/soft
					if t <= 0 {
						continue
					}
					k = int32(strength*t*t*(3-2*t)*256 + 0.5)
				}
			}
			p := row[x*4 : x*4+3 : x*4+3]
			p[0] = mix(p[0], bg.R, k)
			p[1] = mix(p[1], bg.G, k)
			p[2] = mix(p[2], bg.B, k)
		}
	}
}

func mix(a, b uint8, k int32) uint8 {
	return uint8(int32(a) + ((int32(b)-int32(a))*k+128)>>8)
}
