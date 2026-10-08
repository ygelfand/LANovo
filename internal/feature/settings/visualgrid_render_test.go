package settings

import (
	"testing"

	sharedvisual "github.com/ygelfand/libcountertop/pkg/display/visual"

	"github.com/ygelfand/LANovo/internal/ui/visual"
)

func TestEveryVisualHasAThumbnail(t *testing.T) {
	for _, k := range sharedvisual.Built() {
		for _, portrait := range []bool{false, true} {
			if visual.Thumbs().Thumbnail(k, portrait) == nil {
				t.Errorf("%s has no thumbnail (portrait %v); run make thumbs", k, portrait)
			}
		}
	}
}
