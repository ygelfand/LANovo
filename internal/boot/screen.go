package boot

import (
	"github.com/ygelfand/libcountertop/pkg/say"
	"golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	bootview "github.com/ygelfand/libcountertop/pkg/display/boot"
	"github.com/ygelfand/libcountertop/pkg/display/style"
	"github.com/ygelfand/libcountertop/pkg/runtime/startup"
)

// How the screen is divided, and how big the lettering is, as fractions.
const (
	rowShare     = 0.090
	labelShare   = 0.30
	doingShare   = 0.24
	markShare    = 0.24
	versionShare = 0.024
	skipShare    = 0.09

	// The restarting screen: a smaller mark than the boot logo, to leave the word room under it.
	leavingMarkShare = 0.34
	leavingWordShare = 0.045

	strandedSignShare = 0.28
	strandedWordShare = 0.045
)

// Leaving is what the screen says on the way out.
//
// A function rather than a constant because it is translated, and it is read at the moment the
// screen is drawn: by then the saved language has been applied.
func Leaving() string { return say.T("boot.leaving") }

// DrawLogo paints the mark alone, which is what the device shows while it is still finding its
// feet.
func DrawLogo(p *display.Panel) error { return drawLogo(ui.Of(p)) }

// DrawBoot paints the mark and what the device is still waiting for.
func DrawBoot(p *display.Panel, progress []component.Progress) error {
	return drawBoot(ui.Of(p), progress)
}

// chosen is the theme to draw in, so a dark device does not flash white while it starts. A missing
// file, an unreadable one and an unknown name all land on the default, which is light.
func chosen() theme.Theme {
	s := config.Get().Screen
	t, ok := theme.ByName(style.Theme(s.Style, s.Theme))
	if !ok {
		return theme.Default()
	}
	return t
}

// DrawLeaving paints the mark with the word under it, which is what the process puts up as it
// goes. The framebuffer holds it through the gap, so the next one is not read as a cold boot.
func DrawLeaving(p *display.Panel) error { return drawLeaving(ui.Of(p)) }

func drawLeaving(s ui.Surface) error {
	w, h := s.Size()

	palette := chosen()
	ui.Fill(s, palette.Background)

	short := min(w, h)
	mark := int(float64(short) * leavingMarkShare)

	font := ui.MustLoad(ui.Medium, int(float64(short)*leavingWordShare))
	wordWidth, wordHeight := font.Measure(Leaving())

	// The mark and the line under it centered together, rather than the mark centered with the
	// line hung off it: the pair is what is being looked at.
	gap := wordHeight
	top := (h - (mark + gap + wordHeight)) / 2

	ui.DrawLogo(s, ui.Rect{X: (w - mark) / 2, Y: top, W: mark, H: mark}, palette.Background)

	// Larger than the lettering it sits beside: a Material icon is drawn with padding inside its
	// box, so matching the box makes the glyph look smaller than the text.
	glyph := wordHeight * 5 / 4
	gutter := wordHeight / 2
	line := glyph + gutter + wordWidth

	x, y := (w-line)/2, top+mark+gap
	ui.DrawIcon(s, icons.ActionAutorenew,
		ui.Rect{X: x, Y: y + (wordHeight-glyph)/2, W: glyph, H: glyph},
		palette.Muted, palette.Background)

	ui.DrawText(s, font, x+glyph+gutter, y, palette.Muted, palette.Background, Leaving())

	drawVersion(s, palette)
	return nil
}

func Stranded() string { return say.T("boot.stranded") }

func DrawStranded(p *display.Panel) error { return drawStranded(ui.Of(p)) }

func drawStranded(s ui.Surface) error {
	w, h := s.Size()

	palette := chosen()
	ui.Fill(s, palette.Background)

	short := min(w, h)
	mark := int(float64(short) * leavingMarkShare)
	sign := int(float64(short) * strandedSignShare)

	font := ui.MustLoad(ui.Medium, int(float64(short)*strandedWordShare))
	wordWidth, wordHeight := font.Measure(Stranded())

	gap := wordHeight
	top := (h - (mark + gap + sign + gap + wordHeight)) / 2

	ui.DrawLogo(s, ui.Rect{X: (w - mark) / 2, Y: top, W: mark, H: mark}, palette.Background)
	ui.DrawIcon(s, icons.AlertWarning,
		ui.Rect{X: (w - sign) / 2, Y: top + mark + gap, W: sign, H: sign},
		palette.Danger, palette.Background)
	ui.DrawText(s, font, (w-wordWidth)/2, top+mark+gap+sign+gap, palette.Text, palette.Background, Stranded())

	drawVersion(s, palette)
	return nil
}

func drawLogo(s ui.Surface) error {
	w, h := s.Size()
	ui.Clear(s, ui.Rect{W: w, H: h})
	drawVersion(s, chosen())
	return nil
}

func drawVersion(s ui.Surface, palette theme.Theme) { bootview.Version(s, palette, layout.Version) }
func drawBoot(s ui.Surface, progress []component.Progress) error {
	bootview.Raster(s, chosen(), progress, layout.Version, say.T("boot.skip"))
	return nil
}
func Settled(progress []component.Progress) bool { return startup.Settled(progress) }
func Skip(w, h int) ui.Rect                      { return bootview.Skip(w, h) }
