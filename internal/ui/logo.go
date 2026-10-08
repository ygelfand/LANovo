package ui

import (
	"bytes"
	_ "embed"
	"image"
	_ "image/png"
	"sync"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

//go:embed logo_light.png
var logoPNG []byte

//go:embed logo_dark.png
var nightPNG []byte

var (
	logoOnce sync.Once
	logo     image.Image

	nightOnce sync.Once
	night     image.Image
)

func Logo() image.Image { return decode(&logoOnce, &logo, logoPNG) }

func LogoPNG() []byte { return logoPNG }

func Night() image.Image { return decode(&nightOnce, &night, nightPNG) }

var (
	nightSurfaceOnce sync.Once
	nightSurface     *Image
)

func NightSurface() *Image {
	nightSurfaceOnce.Do(func() { nightSurface, _ = Decode(nightPNG) })
	return nightSurface
}

func decode(once *sync.Once, into *image.Image, raw []byte) image.Image {
	once.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(raw))
		if err == nil {
			*into = img
		}
	})
	return *into
}

func MarkWidth(h int) int {
	img := Logo()
	if img == nil {
		return h
	}

	b := img.Bounds()
	if b.Dy() == 0 {
		return h
	}
	return b.Dx() * h / b.Dy()
}

func DrawLogo(s Surface, r Rect, on theme.Color) {
	img := Logo()
	if theme.Dark(on) {
		img = Night()
	}
	DrawImageFit(s, img, r, on)
}
