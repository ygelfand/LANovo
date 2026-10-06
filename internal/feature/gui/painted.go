package gui

import (
	"fmt"

	gogui "github.com/go-gui-org/go-gui/gui"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/libcountertop/pkg/display/style"
)

type paint func(s ui.Surface, at ui.Rect, palette theme.Theme)

func palette() theme.Theme {
	s := config.Get().Screen
	if p, ok := theme.ByName(style.Theme(s.Style, s.Theme)); ok {
		return p
	}
	return theme.Default()
}

func painted(key string, w, h int, fill theme.Color, draw paint) string {
	p := palette()
	key = fmt.Sprintf("%s/%s/%dx%d", key, p.Name, w, h)
	if gogui.HasImage(key) {
		return "mem:" + key
	}
	img := ui.NewImage(w, h, fill)
	draw(img, ui.Rect{W: w, H: h}, p)
	return gogui.UseImage(key, w, h, nrgba(img))
}

func nrgba(img *ui.Image) []byte {
	w, h := img.Size()
	pix := make([]byte, 0, w*h*4)
	for y := range h {
		for x := range w {
			c := img.At(x, y)
			pix = append(pix, c.R, c.G, c.B, 255)
		}
	}
	return pix
}

func picture(src string, w, h int) gogui.View {
	return gogui.Image(gogui.ImageCfg{Src: src, Width: float32(w), Height: float32(h)})
}
