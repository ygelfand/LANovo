package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	cell   = 64
	across = 8
)

var runes = []rune("ｦｱｳｴｵｶｷｹｺｻｼｽｾｿﾀﾂﾃﾅﾆﾇﾈﾊﾋﾎﾏﾐﾑﾒﾓﾔﾕﾗﾘﾜｲｸﾁﾄﾉﾌﾍﾖﾙﾚﾛﾝ0123456789Z:・=*+<¦")

func main() {
	in := flag.String("in", "", "font to render from")
	out := flag.String("out", "internal/ui/visual/matrix/glyphs.png", "atlas to write")
	bold := flag.Int("bold", 2, "pixels to thicken each stroke by")
	flag.Parse()
	if *in == "" {
		flag.Usage()
		os.Exit(2)
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	f, err := opentype.Parse(data)
	if err != nil {
		log.Fatal(err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: cell * 0.78, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		log.Fatal(err)
	}
	if len(runes) > across*across {
		log.Fatalf("%d glyphs, room for %d", len(runes), across*across)
	}
	atlas := image.NewGray(image.Rect(0, 0, cell*across, cell*across))
	m := face.Metrics()
	for i, r := range runes {
		if _, ok := face.GlyphAdvance(r); !ok {
			log.Fatalf("%q missing from %s", r, *in)
		}
		one := image.NewGray(image.Rect(0, 0, cell, cell))
		b, adv := font.BoundString(face, string(r))
		w := (b.Max.X - b.Min.X).Ceil()
		x := (cell-w)/2 - b.Min.X.Floor()
		if w <= 0 {
			x = (cell - adv.Ceil()) / 2
		}
		y := (cell+(m.Ascent-m.Descent).Ceil())/2 - 2
		d := font.Drawer{Dst: one, Src: image.NewUniform(color.Gray{Y: 255}), Face: face, Dot: fixed.P(x, y)}
		d.DrawString(string(r))
		ox, oy := (i%across)*cell, (i/across)*cell
		for py := range cell {
			for px := range cell {
				atlas.SetGray(ox+cell-1-px, oy+py, color.Gray{Y: thick(one, px, py, *bold)})
			}
		}
	}
	if err := os.MkdirAll("internal/ui/visual/matrix", 0o755); err != nil {
		log.Fatal(err)
	}
	fo, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer fo.Close()
	if err := png.Encode(fo, atlas); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d glyphs to %s", len(runes), *out)
}

func thick(g *image.Gray, x, y, r int) uint8 {
	var best uint8
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy > r*r {
				continue
			}
			if v := g.GrayAt(x+dx, y+dy).Y; v > best {
				best = v
			}
		}
	}
	return best
}
