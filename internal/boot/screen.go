package boot

import (
	"golang.org/x/exp/shiny/materialdesign/icons"

	bootview "github.com/ygelfand/libcountertop/pkg/display/boot"
	"github.com/ygelfand/libcountertop/pkg/display/panel"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	"github.com/ygelfand/libcountertop/pkg/display/theme"
	sharedui "github.com/ygelfand/libcountertop/pkg/display/ui"
	"github.com/ygelfand/libcountertop/pkg/say"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/ui"
)

const (
	leavingMarkShare = 0.34
	leavingWordShare = 0.045
)

func Leaving() string { return say.T("boot.leaving") }

func DrawLogo(p *panel.Panel) error { return drawLogo(sharedui.Of(p)) }

func chosen() theme.Theme {
	s := config.Get().Screen
	t, ok := theme.ByName(style.Theme(s.Style, s.Theme))
	if !ok {
		return theme.Default()
	}
	return t
}

func DrawLeaving(p *panel.Panel) error { return drawLeaving(sharedui.Of(p)) }

func drawLeaving(s sharedui.Surface) error {
	w, h := s.Size()

	palette := chosen()
	sharedui.Fill(s, palette.Background)

	short := min(w, h)
	mark := int(float64(short) * leavingMarkShare)

	font := sharedui.MustLoad(sharedui.Medium, int(float64(short)*leavingWordShare))
	wordWidth, wordHeight := font.Measure(Leaving())

	gap := wordHeight
	top := (h - (mark + gap + wordHeight)) / 2

	ui.Logo().
		Draw(s, sharedui.Rect{X: (w - mark) / 2, Y: top, W: mark, H: mark}, palette.Background)

	// A Material icon is drawn with padding inside its box.
	glyph := wordHeight * 5 / 4
	gutter := wordHeight / 2
	line := glyph + gutter + wordWidth

	x, y := (w-line)/2, top+mark+gap
	sharedui.DrawIcon(s, icons.ActionAutorenew,
		sharedui.Rect{X: x, Y: y + (wordHeight-glyph)/2, W: glyph, H: glyph},
		palette.Muted, palette.Background)

	sharedui.DrawText(s, font, x+glyph+gutter, y, palette.Muted, palette.Background, Leaving())

	drawVersion(s, palette)
	return nil
}

func DrawStranded(p *panel.Panel) error {
	bootview.Stranded(sharedui.Of(p), chosen(), ui.Logo().Draw, layout.Version)
	return nil
}

func drawLogo(s sharedui.Surface) error {
	w, h := s.Size()
	sharedui.Clear(s, sharedui.Rect{W: w, H: h})
	drawVersion(s, chosen())
	return nil
}

func drawVersion(s sharedui.Surface, palette theme.Theme) {
	bootview.Version(s, palette, layout.Version)
}
