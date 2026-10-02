package visual

import (
	"image"
	"image/color"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	typefaces            sync.Once
	boldFace, mediumFace *opentype.Font
)

func faces() (bold, medium *opentype.Font) {
	typefaces.Do(func() {
		boldFace, _ = opentype.Parse(gobold.TTF)
		mediumFace, _ = opentype.Parse(gomedium.TTF)
	})
	return boldFace, mediumFace
}

var (
	monofaces              sync.Once
	monoFace, monoBoldFace *opentype.Font
)

func monoFaces() (regular, bold *opentype.Font) {
	monofaces.Do(func() {
		monoFace, _ = opentype.Parse(gomono.TTF)
		monoBoldFace, _ = opentype.Parse(gomonobold.TTF)
	})
	return monoFace, monoBoldFace
}

func letterAt(img *image.RGBA, f *opentype.Font, size float64, c color.NRGBA, s string, x, y, anchor float64) float64 {
	if f == nil || s == "" || size < 1 {
		return 0
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return 0
	}
	defer face.Close()
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face}
	w := d.MeasureString(s)
	d.Dot = fixed.Point26_6{X: fixed.Int26_6((x - float64(w)/64*anchor) * 64), Y: fixed.Int26_6((y+size/2)*64) - face.Metrics().Descent}
	d.DrawString(s)
	return float64(w) / 64
}

func letter(img *image.RGBA, f *opentype.Font, size float64, c color.NRGBA, s string, x, y float64) {
	letterWithin(img, f, size, c, s, x, y, 0)
}

func letterWithin(img *image.RGBA, f *opentype.Font, size float64, c color.NRGBA, s string, x, y, most float64) {
	if f == nil || s == "" || size < 1 {
		return
	}
	if most > 0 {
		if w := measure(f, size, s); w > most {
			size *= most / w
		}
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return
	}
	defer face.Close()
	m := face.Metrics()
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face}
	w := d.MeasureString(s)
	mid := (m.Ascent - m.Descent) / 2
	d.Dot = fixed.Point26_6{X: fixed.Int26_6(x*64) - w/2, Y: fixed.Int26_6(y*64) + mid}
	d.DrawString(s)
}

func measure(f *opentype.Font, size float64, s string) float64 {
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return 0
	}
	defer face.Close()
	return float64(font.MeasureString(face, s)) / 64
}
