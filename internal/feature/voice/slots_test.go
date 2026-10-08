package voice

import (
	"slices"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/inference/wake"
	"github.com/ygelfand/libcountertop/pkg/inference/wakeslots"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
)

func TestSavedSecondaryAndExplicitDisableSurviveStartupSelection(t *testing.T) {
	config.Use(t.TempDir() + "/state.json")
	if err := config.Set().Wake(1).ID("B"); err != nil {
		t.Fatal(err)
	}
	installed := []wake.Model{{ID: "B", Phrase: "B"}, {ID: wake.DefaultModel, Phrase: "default"}}
	if got := wanted(installed, wakeword.Slots); !slices.Equal(got, []string{"", "B"}) {
		t.Fatal(got)
	}
	if err := config.Set().Wake(1).ID(""); err != nil {
		t.Fatal(err)
	}
	if got := wakeslots.Advertised(wanted(installed, wakeword.Slots)); len(got) != 0 {
		t.Fatal("disabled selection re-enabled", got)
	}
}
