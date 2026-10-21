package all

import (
	"strings"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/runtime/webapi"

	"github.com/ygelfand/LANovo/internal/component"
)

func TestEverySettingHasItsLabels(t *testing.T) {
	entities(t)
	if missing := component.Settings.Unlabeled(); len(missing) > 0 {
		t.Errorf("en.yaml has no text for:\n\t%s", strings.Join(missing, "\n\t"))
	}
	if loose := webapi.Unplaced(&component.Settings); len(loose) > 0 {
		t.Errorf("webapi sections hold no place for:\n\t%s", strings.Join(loose, "\n\t"))
	}
}
