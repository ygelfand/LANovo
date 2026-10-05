package style

import (
	"testing"

	gogui "github.com/go-gui-org/go-gui/gui"
)

func TestStyleNamesAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range All {
		if seen[s.Name] {
			t.Errorf("two styles are called %q", s.Name)
		}
		seen[s.Name] = true
		if s.Kit == nil || s.Shape == nil || s.Family == "" {
			t.Errorf("%s is missing its kit, shape or typeface", s.Name)
		}
	}
	if ByName("no-such-style").Name != Standard {
		t.Error("an unknown style does not fall back to the standard one")
	}
}

func TestEveryStyleDrawsEveryElement(t *testing.T) {
	marks := []Span{{From: 0.2, To: 0.3, Color: gogui.RGB(0xff, 0x00, 0x00)}}
	for _, s := range All {
		k := s.Kit
		for _, on := range []bool{false, true} {
			if k.Toggle("t", on) == nil {
				t.Errorf("%s: no toggle (on=%v)", s.Name, on)
			}
		}
		for _, vertical := range []bool{false, true} {
			cfg := gogui.SliderCfg{Vertical: vertical}
			k.Slider(&cfg)
			if cfg.Look != nil {
				parts := cfg.Look(gogui.SliderLookState{Pct: 0.4})
				if parts.Track == nil || parts.Handle == nil {
					t.Errorf("%s: slider (vertical=%v) is missing a part", s.Name, vertical)
				}
			}
		}
		parts := k.Seek(marks)(gogui.SliderLookState{Pct: 0.5})
		if parts.Track == nil || parts.Handle == nil {
			t.Errorf("%s: seek bar is missing a part", s.Name)
		}
		k.Progress(&gogui.ProgressBarCfg{Percent: 0.5})
		k.Field(&gogui.InputCfg{})
		k.Panel(&gogui.ContainerCfg{})
		for _, st := range []State{Rest, Partial, Chosen} {
			text := gogui.CurrentTheme().Cfg.TextStyleDef
			k.Key(&gogui.ContainerCfg{Width: 40, Height: 40}, &text, st)
			k.Tab(&gogui.ContainerCfg{Height: 40}, &text, st)
			k.Tile(&gogui.ContainerCfg{}, st)
			k.Row(&gogui.ContainerCfg{}, st)
		}
	}
}
