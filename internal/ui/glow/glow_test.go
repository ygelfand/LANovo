package glow

import (
	"image"
	"image/color"
	"testing"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

var rgbGrey = theme.Color{R: 50, G: 50, B: 50}

func at(img *image.RGBA, x, y int) (r, g, b uint8) {
	i := y*img.Stride + x*4
	return img.Pix[i], img.Pix[i+1], img.Pix[i+2]
}

func TestClearFillsEveryPixel(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 37, 11))
	Clear(img, color.NRGBA{10, 20, 30, 255})
	for _, p := range [][2]int{{0, 0}, {36, 10}, {18, 5}} {
		if r, g, b := at(img, p[0], p[1]); r != 10 || g != 20 || b != 30 {
			t.Errorf("(%d,%d) = %d,%d,%d", p[0], p[1], r, g, b)
		}
	}
}

func TestRadialRunsFromCentreToEdge(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 101, 101))
	Clear(img, color.NRGBA{0, 0, 0, 255})
	Radial(img, 50.5, 50.5, 0, 50, []Stop{{0, color.NRGBA{255, 0, 0, 255}}, {1, color.NRGBA{255, 0, 0, 0}}}, false)
	if r, _, _ := at(img, 50, 50); r < 250 {
		t.Errorf("centre red %d, want full", r)
	}
	if r, _, _ := at(img, 75, 50); r < 100 || r > 155 {
		t.Errorf("half way red %d, want about half", r)
	}
	if r, _, _ := at(img, 0, 0); r != 0 {
		t.Errorf("corner red %d, want none", r)
	}
}

func TestAdditiveSaturates(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	Clear(img, color.NRGBA{200, 10, 0, 255})
	src := image.NewRGBA(image.Rect(0, 0, 3, 3))
	Clear(src, color.NRGBA{100, 10, 0, 255})
	Add(img, src, 1)
	if r, g, _ := at(img, 1, 1); r != 255 || g != 20 {
		t.Errorf("added to %d,%d, want 255,20", r, g)
	}
}

func TestBloomSpreadsABrightPoint(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	Clear(img, color.NRGBA{0, 0, 0, 255})
	for y := 96; y < 104; y++ {
		for x := 96; x < 104; x++ {
			img.Pix[y*img.Stride+x*4] = 255
		}
	}
	var bl Bloom
	bl.Apply(img, 1, 0.05, 2)
	if r, _, _ := at(img, 115, 100); r == 0 {
		t.Error("no light reached beside the point")
	}
	if r, _, _ := at(img, 5, 5); r != 0 {
		t.Errorf("light reached the far corner: %d", r)
	}
	if _, g, _ := at(img, 100, 100); g != 0 {
		t.Errorf("bloom invented green: %d", g)
	}
}

func TestBlitScalesIntoTheBox(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	Clear(img, color.NRGBA{0, 0, 0, 255})
	img.Pix[0] = 255
	dst := ui.NewImage(10, 10, rgbGrey)
	Blit(dst, img, ui.Rect{X: 3, Y: 3, W: 4, H: 4}, 2)
	if c := dst.At(4, 4); c.R != 255 {
		t.Errorf("scaled pixel = %v", c)
	}
	if c := dst.At(5, 5); c.R != 0 {
		t.Errorf("neighbour = %v", c)
	}
	if c := dst.At(2, 2); c != rgbGrey {
		t.Errorf("outside the box changed to %v", c)
	}
}

func BenchmarkClearFull(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 1920))
	for b.Loop() {
		Clear(img, color.NRGBA{2, 3, 10, 255})
	}
}

func BenchmarkRadialHalf(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 600, 960))
	stops := []Stop{{0, color.NRGBA{70, 200, 255, 64}}, {1, color.NRGBA{70, 200, 255, 0}}}
	for b.Loop() {
		Radial(img, 300, 400, 0, 720, stops, false)
	}
}

func BenchmarkBloomHalf(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 600, 960))
	var bl Bloom
	for b.Loop() {
		bl.Apply(img, 0.6, 0.02, 2)
	}
}
