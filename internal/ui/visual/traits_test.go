package visual

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func TestEveryKindDeclaresItsTraits(t *testing.T) {
	for _, k := range Kinds() {
		if _, ok := traits[k]; !ok {
			t.Errorf("%s has no traits", k)
		}
	}
}

func TestTheClockOverAVisualReadsOnIt(t *testing.T) {
	light := theme.Default().On(false)
	if got := (Traits{}).Under(light); !got.Dark || theme.Contrast(got.Text, got.Background) < 4.5 {
		t.Errorf("over a dark visual the clock is %+v", got)
	}
	dark := theme.Default().On(true)
	if got := (Traits{Light: true}).Under(dark); got.Dark {
		t.Errorf("over a light visual the clock palette is dark")
	}
	if got := (Traits{Themed: true}).Under(light); got != light {
		t.Error("a themed visual changed the clock's palette")
	}
}
