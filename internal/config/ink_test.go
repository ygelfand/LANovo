package config

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func TestTheDefaultInkChangesNothing(t *testing.T) {
	for _, palette := range theme.All {
		if got := DefaultInk.Over(palette); got != palette {
			t.Errorf("%s: the default ink returned a different theme", palette.Name)
		}
	}
}

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

func TestAnUnknownInkChangesNothing(t *testing.T) {
	palette := theme.Default()

	if got := Ink("chartreuse").Over(palette); got != palette {
		t.Error("an unknown ink changed the theme")
	}
}

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
