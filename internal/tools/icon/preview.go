package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"golang.org/x/exp/shiny/iconvg"
)

func preview(icons []icon, path string, size int) error {
	pad := size / 4
	sheet := image.NewRGBA(image.Rect(0, 0, len(icons)*(size+pad)+pad, size+pad*2))
	draw.Draw(
		sheet,
		sheet.Bounds(),
		&image.Uniform{color.RGBA{0xf5, 0xf5, 0xf5, 0xff}},
		image.Point{},
		draw.Src,
	)

	for i, ic := range icons {
		one := image.NewRGBA(image.Rect(0, 0, size, size))

		var z iconvg.Rasterizer
		z.SetDstImage(one, one.Bounds(), draw.Src)

		opts := iconvg.DecodeOptions{
			Palette: &iconvg.Palette{0: color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}},
		}
		if err := iconvg.Decode(&z, ic.bytes, &opts); err != nil {
			return err
		}

		at := image.Rect(pad+i*(size+pad), pad, pad+i*(size+pad)+size, pad+size)
		draw.DrawMask(sheet, at, &image.Uniform{color.RGBA{0x11, 0x11, 0x11, 0xff}},
			image.Point{}, one, image.Point{}, draw.Over)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, sheet)
}
