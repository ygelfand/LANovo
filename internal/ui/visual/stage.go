package visual

import (
	"image"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type stage struct {
	w, h int
	u    float64

	back    *image.RGBA
	backKey uint64
}

func (st *stage) measure(box ui.Rect) bool {
	if box.W < 2 || box.H < 2 {
		return false
	}
	st.w, st.h = box.W, box.H
	st.u = float64(min(box.W, box.H)) / 800
	return true
}

func (st *stage) repaint(key uint64, draw func(*image.RGBA)) {
	bounds := image.Rect(0, 0, st.w, st.h)
	if st.back != nil && st.back.Bounds() == bounds && st.backKey == key {
		return
	}
	if st.back == nil || st.back.Bounds() != bounds {
		st.back = image.NewRGBA(bounds)
	}
	draw(st.back)
	st.backKey = key
}

type rgbaSurface struct{ img *image.RGBA }

func (r rgbaSurface) Size() (int, int) { return r.img.Bounds().Dx(), r.img.Bounds().Dy() }

func (r rgbaSurface) Set(x, y int, c theme.Color) {
	if !(image.Point{X: x, Y: y}).In(r.img.Bounds()) {
		return
	}
	i := y*r.img.Stride + x*4
	r.img.Pix[i], r.img.Pix[i+1], r.img.Pix[i+2] = c.R, c.G, c.B
}

func (r rgbaSurface) At(x, y int) theme.Color {
	if !(image.Point{X: x, Y: y}).In(r.img.Bounds()) {
		return theme.Color{}
	}
	i := y*r.img.Stride + x*4
	return theme.Color{R: r.img.Pix[i], G: r.img.Pix[i+1], B: r.img.Pix[i+2]}
}
