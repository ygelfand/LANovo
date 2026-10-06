package gui

import (
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/libcountertop/pkg/display/style"
)

func TestEveryStyleThemeAndSizeMakesATheme(t *testing.T) {
	for _, st := range style.Names() {
		for _, p := range theme.Names() {
			for _, sz := range Sizes() {
				th := Look(st, p, sz)
				if want := st + "/" + p + "/" + sz; th.Cfg.Name != want {
					t.Errorf("made %q, want %q", th.Cfg.Name, want)
				}
			}
		}
	}
}

func TestAnUnknownThemeFallsBackToTheDefault(t *testing.T) {
	th := Look(style.Standard, "no-such-palette", SizeCompact)
	if !strings.Contains(th.Cfg.Name, "/"+theme.DefaultName+"/") {
		t.Errorf("got %q, want the default theme %q", th.Cfg.Name, theme.DefaultName)
	}
}

func TestEachStyleBringsItsTypeface(t *testing.T) {
	for _, st := range style.All {
		if got := Look(st.Name, theme.DefaultName, SizeLarge).Cfg.TextStyleDef.Family; got != st.Family {
			t.Errorf("%s draws in %q, want %q", st.Name, got, st.Family)
		}
	}
}

func TestSizesDiffer(t *testing.T) {
	large := Look(style.Standard, "", SizeLarge)
	compact := Look(style.Standard, "", SizeCompact)
	if large.Cfg.SizeTextMedium <= compact.Cfg.SizeTextMedium {
		t.Errorf("large text %v is not bigger than compact %v", large.Cfg.SizeTextMedium, compact.Cfg.SizeTextMedium)
	}
}

func TestDefaultTakesEachStylesPalette(t *testing.T) {
	for _, st := range style.All {
		if _, ok := theme.ByName(st.Palette); !ok {
			t.Errorf("%s recommends %q, which is not a theme", st.Name, st.Palette)
		}
		if got := Look(st.Name, style.ThemeDefault, SizeLarge).Cfg.Name; !strings.Contains(got, "/"+st.Palette+"/") {
			t.Errorf("%s with the default theme made %q, want its palette %q", st.Name, got, st.Palette)
		}
	}
}

func TestSizesGrowInOrder(t *testing.T) {
	var last float32
	for _, sz := range Sizes() {
		body := Look(style.Standard, "", sz).Cfg.TextStyleDef.Size
		if body <= last {
			t.Errorf("%s body text %v is not bigger than the size before it (%v)", sz, body, last)
		}
		last = body
	}
}
