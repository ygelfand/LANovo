package theme

import "testing"

func TestByName(t *testing.T) {
	got, ok := ByName("Midnight")
	if !ok {
		t.Fatal("Midnight is not among the themes")
	}
	if got.Name != "Midnight" {
		t.Errorf("ByName returned %q", got.Name)
	}

	if _, ok := ByName("Nonesuch"); ok {
		t.Error("an unknown name resolved to a theme")
	}
}

func TestDefaultExists(t *testing.T) {
	if _, ok := ByName(DefaultName); !ok {
		t.Fatalf("the default theme %q is not among them", DefaultName)
	}
	if Default().Name != DefaultName {
		t.Errorf("Default() is %q, want %q", Default().Name, DefaultName)
	}
}

func TestNamesCoversEveryTheme(t *testing.T) {
	names := Names()
	if len(names) != len(All) {
		t.Fatalf("%d names for %d themes", len(names), len(All))
	}
	for i, name := range names {
		if name != All[i].Name {
			t.Errorf("name %d is %q, want %q", i, name, All[i].Name)
		}
	}
}

// Two themes with one name is a select with a row that cannot be chosen.
func TestNamesAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range Names() {
		if seen[name] {
			t.Errorf("two themes are called %q", name)
		}
		seen[name] = true
	}
}

func TestEnoughThemesToChooseFrom(t *testing.T) {
	if len(All) < 12 {
		t.Errorf("%d themes, want at least a dozen", len(All))
	}
}

// Text on background is the one pairing every screen uses, so it has to be readable in all of
// them. The ratio is the WCAG contrast formula; 4.5 is its threshold for body text.
func TestTextIsReadableOnBackground(t *testing.T) {
	for _, th := range All {
		if r := Contrast(th.Text, th.Background); r < 4.5 {
			t.Errorf("%s: text on background is %.1f:1, want at least 4.5:1", th.Name, r)
		}
	}
}

// Muted text is secondary, so it may be softer — but it still has to be legible across a room.
func TestMutedIsLegibleOnBackground(t *testing.T) {
	for _, th := range All {
		if r := Contrast(th.Muted, th.Background); r < 3 {
			t.Errorf("%s: muted on background is %.1f:1, want at least 3:1", th.Name, r)
		}
	}
}

func TestTextIsReadableOnSurface(t *testing.T) {
	for _, th := range All {
		if r := Contrast(th.Text, th.Surface); r < 4.5 {
			t.Errorf("%s: text on surface is %.1f:1, want at least 4.5:1", th.Name, r)
		}
	}
}

// A surface has to be distinguishable from the background it sits on, or a card has no edges.
func TestSurfaceIsDistinctFromBackground(t *testing.T) {
	for _, th := range All {
		if th.Surface == th.Background {
			t.Errorf("%s: surface and background are the same color", th.Name)
		}
	}
}

// Dark says which way round a theme is, and the drawing code uses it to decide things like
// whether a shadow or a glow is wanted.
func TestDarkMatchesTheBackground(t *testing.T) {
	for _, th := range All {
		light := luminance(th.Background) > 0.5
		if th.Dark == light {
			t.Errorf("%s: Dark is %v but its background luminance is %.2f",
				th.Name, th.Dark, luminance(th.Background))
		}
	}
}

// Contrast picks what goes on an accent. Accents carry large text and shapes rather than body
// copy, which is the 3:1 threshold rather than 4.5:1.
func TestContrastIsReadableOnEveryAccent(t *testing.T) {
	for _, th := range All {
		for _, on := range []struct {
			name string
			c    Color
		}{
			{"accent", th.Accent}, {"accent2", th.Accent2},
			{"success", th.Success}, {"warning", th.Warning}, {"danger", th.Danger},
		} {
			if r := Contrast(th.Contrast(on.c), on.c); r < 3 {
				t.Errorf("%s: contrast on %s is %.1f:1, want at least 3:1", th.Name, on.name, r)
			}
		}
	}
}

func TestBlend(t *testing.T) {
	black, white := Color{}, Color{R: 255, G: 255, B: 255}

	if got := black.Blend(white, 0); got != black {
		t.Errorf("blending none of the way gave %v, want the original", got)
	}
	if got := black.Blend(white, 1); got != white {
		t.Errorf("blending all the way gave %v, want the other color", got)
	}

	half := black.Blend(white, 0.5)
	if half.R < 120 || half.R > 136 {
		t.Errorf("blending half way gave %v, want about half", half)
	}

	// Out of range must not wrap round to a wildly different color.
	if got := black.Blend(white, -1); got != black {
		t.Errorf("a negative amount gave %v, want the original", got)
	}
	if got := black.Blend(white, 2); got != white {
		t.Errorf("an amount past one gave %v, want the other color", got)
	}
}

func TestColorString(t *testing.T) {
	if got := rgb(0x0b0e14).String(); got != "#0b0e14" {
		t.Errorf("String() = %q, want #0b0e14", got)
	}
}

func TestAThemeRetonedForABackdropReadsOnIt(t *testing.T) {
	for _, th := range All {
		for _, dark := range []bool{true, false} {
			on := th.On(dark)
			if on.Dark != dark || Dark(on.Background) != dark {
				t.Errorf("%s on dark=%v: background %v reads the wrong way", th.Name, dark, on.Background)
			}
			if c := Contrast(on.Text, on.Background); c < 4.5 {
				t.Errorf("%s on dark=%v: text contrast %.1f", th.Name, dark, c)
			}
			if on.Accent != th.Accent {
				t.Errorf("%s on dark=%v: accent moved", th.Name, dark)
			}
		}
		if th.On(th.Dark) != th {
			t.Errorf("%s: retoning to its own way round changed it", th.Name)
		}
	}
}
