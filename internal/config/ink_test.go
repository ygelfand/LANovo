package config

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// The default has to leave the theme exactly as it was. It is the instruction not to recolor
// anything, and a theme that came back subtly different would be a device that looks changed the
// moment the setting exists.
func TestTheDefaultInkChangesNothing(t *testing.T) {
	for _, palette := range theme.All {
		if got := DefaultInk.Over(palette); got != palette {
			t.Errorf("%s: the default ink returned a different theme", palette.Name)
		}
	}
}

// Every other ink has to actually recolor, or it is a row in a list that does nothing.
func TestEveryInkRecolorsTheText(t *testing.T) {
	palette := theme.Default()

	for _, ink := range Inks() {
		if ink == InkTheme {
			continue
		}

		got := ink.Over(palette)
		if got.Text == palette.Text {
			t.Errorf("%s left the text alone", ink)
		}
		if got.Muted == palette.Muted {
			t.Errorf("%s left the date alone", ink)
		}
	}
}

// The roles the clock does not draw itself in are left alone. Accent is the second hand and has to
// stay apart from the other two; Surface is the card the cards face sits on and the unlit lamps
// behind the segments, which are the object rather than the clock.
func TestInkLeavesTheOtherRolesAlone(t *testing.T) {
	palette := theme.Default()

	for _, ink := range Inks() {
		got := ink.Over(palette)

		switch {
		case got.Accent != palette.Accent:
			t.Errorf("%s changed the accent", ink)
		case got.Surface != palette.Surface:
			t.Errorf("%s changed the surface", ink)
		case got.Background != palette.Background:
			t.Errorf("%s changed the background", ink)
		case got.Name != palette.Name:
			t.Errorf("%s renamed the theme", ink)
		}
	}
}

// The date has to sit apart from the time, the way it does in every theme: the same color for both
// is a date that reads as part of the time.
func TestTheDateIsSoftenedFromTheTime(t *testing.T) {
	for _, palette := range theme.All {
		for _, ink := range Inks() {
			if ink == InkTheme {
				continue
			}

			got := ink.Over(palette)
			if got.Muted == got.Text {
				t.Errorf("%s on %s: the date is the same color as the time", ink, palette.Name)
			}
		}
	}
}

// A clock has to be readable on every theme, half of which are light and half dark. Four to one is
// the usual floor for large text, and every numeral here is large.
func TestEveryInkReadsOnEveryTheme(t *testing.T) {
	const floor = 4.0

	for _, palette := range theme.All {
		for _, ink := range Inks() {
			got := ink.Over(palette)

			if r := theme.Contrast(got.Text, got.Background); r < floor {
				t.Errorf("%s on %s: the time is %.2f against the background, want %.2f",
					ink, palette.Name, r, floor)
			}
		}
	}
}

// A name this build does not have leaves the theme alone rather than drawing the clock in nothing.
func TestAnUnknownInkChangesNothing(t *testing.T) {
	palette := theme.Default()

	if got := Ink("chartreuse").Over(palette); got != palette {
		t.Error("an unknown ink changed the theme")
	}
}

// Color is what a swatch draws, so it has to answer for the default too rather than a zero color.
func TestTheDefaultInkIsTheThemeText(t *testing.T) {
	for _, palette := range theme.All {
		if got := DefaultInk.Color(palette); got != palette.Text {
			t.Errorf(
				"%s: the default ink is %v, want the theme text %v",
				palette.Name,
				got,
				palette.Text,
			)
		}
	}
}

func TestEveryInkIsLabeled(t *testing.T) {
	for _, ink := range Inks() {
		if ink.Label() == string(ink) {
			t.Errorf("%s has no label of its own", ink)
		}
	}
}
