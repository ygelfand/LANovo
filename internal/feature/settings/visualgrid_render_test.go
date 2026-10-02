package settings

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/ui/visual"
)

func TestEveryVisualHasAThumbnail(t *testing.T) {
	for _, k := range visual.Built() {
		for _, portrait := range []bool{false, true} {
			if visual.Thumbnail(k, portrait) == nil {
				t.Errorf("%s has no thumbnail (portrait %v); run make thumbs", k, portrait)
			}
		}
	}
}
