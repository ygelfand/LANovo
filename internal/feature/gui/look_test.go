package gui

import (
	"strings"
	"testing"
)

func TestEveryStylePaletteAndSizeMakesATheme(t *testing.T) {
	for _, st := range Styles() {
		for _, p := range PalettesFor(st) {
			for _, sz := range Sizes() {
				th := Look(st, p, sz, family)
				if want := st + "/" + p + "/" + sz; th.Cfg.Name != want {
					t.Errorf("made %q, want %q", th.Cfg.Name, want)
				}
			}
		}
	}
}

func TestAPaletteTheStyleDoesNotOfferFallsBackToItsFirst(t *testing.T) {
	th := Look(StyleStandard, "no-such-palette", SizeCompact, family)
	first := PalettesFor(StyleStandard)[0]
	if !strings.Contains(th.Cfg.Name, "/"+first+"/") {
		t.Errorf("got %q, want the first palette %q", th.Cfg.Name, first)
	}
}

func TestSizesDiffer(t *testing.T) {
	large := Look(StyleStandard, "", SizeLarge, family)
	compact := Look(StyleStandard, "", SizeCompact, family)
	if large.Cfg.SizeTextMedium <= compact.Cfg.SizeTextMedium {
		t.Errorf("large text %v is not bigger than compact %v", large.Cfg.SizeTextMedium, compact.Cfg.SizeTextMedium)
	}
}
