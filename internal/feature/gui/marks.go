package gui

import (
	"embed"
	"fmt"
	"strings"

	gogui "github.com/go-gui-org/go-gui/gui"
)

//go:embed marks/*.svg
var marks embed.FS

func hex(c gogui.Color) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

func mark(id, name string, ink, back gogui.Color, side float32) gogui.View {
	b, err := marks.ReadFile("marks/" + name + ".svg")
	if err != nil {
		return gogui.Column(gogui.ContainerCfg{Width: side, Height: side, Sizing: gogui.FixedFixed})
	}
	svg := strings.NewReplacer("INK", hex(ink), "BACK", hex(back)).Replace(string(b))
	return gogui.Svg(gogui.SvgCfg{ID: id + "-" + hex(ink) + hex(back), SvgData: svg, Width: side, Height: side})
}
