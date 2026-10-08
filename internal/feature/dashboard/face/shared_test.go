package face

import (
	"slices"
	"testing"

	shared "github.com/ygelfand/libcountertop/pkg/display/clockface"

	"github.com/ygelfand/LANovo/internal/config"
)

func TestConfiguredFacesMatchSharedRenderers(t *testing.T) {
	offered := config.Faces()
	for _, name := range offered {
		if !slices.Contains(shared.Kinds(), shared.Kind(name)) {
			t.Errorf("configured face %q has no shared renderer", name)
		}
	}
	for _, kind := range shared.Kinds() {
		if !slices.Contains(offered, config.Face(kind)) {
			t.Errorf("shared face %q is not offered", kind)
		}
	}
	if shared.Kind(config.DefaultFace) != shared.Default {
		t.Error("product and shared default faces differ")
	}
}
