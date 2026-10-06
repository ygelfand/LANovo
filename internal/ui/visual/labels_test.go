package visual

import (
	"strings"
	"testing"
)

func TestEveryKindHasALabel(t *testing.T) {
	if len(Kinds()) != 33 {
		t.Errorf("%d kinds, want 33", len(Kinds()))
	}
	for _, k := range Kinds() {
		if k.Label() == "visual."+string(k) {
			t.Errorf("%s has no label", k)
		}
	}
}
func TestTheBuiltVisualsAreAlphabetical(t *testing.T) {
	b := Built()
	for i := 1; i < len(b); i++ {
		if strings.ToLower(b[i-1].Label()) > strings.ToLower(b[i].Label()) {
			t.Errorf("%q comes before %q", b[i-1].Label(), b[i].Label())
		}
	}
}
