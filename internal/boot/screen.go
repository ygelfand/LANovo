package boot

import (
	"github.com/ygelfand/LANovo/internal/lib/say"
	"golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/reveal"
	"github.com/ygelfand/LANovo/internal/ui/style"
	"github.com/ygelfand/LANovo/internal/ui/theme"
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

func drawVersion(s ui.Surface, palette theme.Theme) {
	w, h := s.Size()

	font := ui.MustLoad(ui.Regular, int(float64(min(w, h))*versionShare))
	tw, th := font.Measure(layout.Version)

	pad := th
	ui.DrawText(s, font, w-tw-pad, h-th-pad, palette.Muted, palette.Background, layout.Version)
}

// drawBoot paints the mark and the list.
//
// The list sits beside the mark when the picture is wide and under it when it is tall, so the
// device says the same thing whichever way it is stood.
func drawBoot(s ui.Surface, progress []component.Progress) error {
	w, h := s.Size()

	palette := chosen()
	logo, list := reveal.Split(w, h)
	ui.Clear(s, logo)
	ui.FillRect(s, list, palette.Background)

	rows(s, list, palette, progress)
	drawVersion(s, palette)
	if !Settled(progress) {
		drawSkip(s, palette)
	}
	return nil
}

func Settled(progress []component.Progress) bool {
	for _, p := range progress {
		if !p.Done && !p.Failed {
			return false
		}
	}
	return true
}

func Skip(w, h int) ui.Rect {
	short := min(w, h)
	high := int(float64(short) * skipShare)
	wide := high * 3
	pad := int(float64(short) * versionShare * 2)
	return ui.Rect{X: w - wide - pad, Y: h - high - 2*pad, W: wide, H: high}
}

func drawSkip(s ui.Surface, palette theme.Theme) {
	w, h := s.Size()
	at := Skip(w, h)
	ui.FillRounded(s, at, at.H/2, palette.Surface)
	font := ui.MustLoad(ui.Medium, at.H*2/5)
	tw, th := font.Measure(say.T("boot.skip"))
	ui.DrawText(s, font, at.X+(at.W-tw)/2, at.Y+(at.H-th)/2, palette.Text, palette.Surface, say.T("boot.skip"))
}

// rows draws the list, centered in the space it was given.
func rows(s ui.Surface, in ui.Rect, palette theme.Theme, progress []component.Progress) {
	if len(progress) == 0 {
		return
	}

	row := int(float64(min(in.W, in.H)) * rowShare)

	labelFont := ui.MustLoad(ui.Medium, int(float64(row)*labelShare))
	doingFont := ui.MustLoad(ui.Regular, int(float64(row)*doingShare))

	mark := int(float64(row) * markShare)
	perColumn, columnW := len(progress), in.W
	if len(progress)*row > in.H*3/4 {
		perColumn, columnW = (len(progress)+1)/2, in.W/2
	}
	top := in.Y + (in.H-perColumn*row)/2

	for i, at := range progress {
		left := in.X + row/2 + (i/perColumn)*columnW
		y := top + (i%perColumn)*row

		color := palette.Muted
		switch {
		case at.Failed:
			color = palette.Warning
		case at.Done:
			color = palette.Accent
		}
		dot(s, left, y+row/2, mark/2, color, at.Done || at.Failed, palette.Background)

		x := left + mark*2
		ui.DrawText(s, labelFont, x, y+row/8, palette.Text, palette.Background, at.Name)

		if at.Doing != "" {
			said := palette.Muted
			if at.Failed {
				said = palette.Warning
			}
			_, lh := labelFont.Measure(at.Name)
			ui.DrawText(s, doingFont, x, y+row/8+lh, said, palette.Background, at.Doing)
		}
	}
}

// dot is the mark beside a row: filled once the component is ready, a ring while it is not.
func dot(s ui.Surface, cx, cy, r int, c theme.Color, filled bool, back theme.Color) {
	ui.FillRounded(s, ui.Rect{X: cx - r, Y: cy - r, W: r * 2, H: r * 2}, r, c)

	if !filled {
		inner := r / 2
		ui.FillRounded(s,
			ui.Rect{X: cx - inner, Y: cy - inner, W: inner * 2, H: inner * 2}, inner, back)
	}
}
