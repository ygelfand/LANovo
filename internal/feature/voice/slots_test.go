package voice

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/lib/wake"
	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/libcountertop/pkg/assistant/turn"
	"github.com/ygelfand/libcountertop/pkg/inference/wakeslots"
	"slices"
	"testing"
)

func TestSecondaryPhraseUsesItsSlotRatherThanTheCompactAdvertisement(t *testing.T) {
	var selected wakeslots.Selection
	selected.Set([]string{"", "B"})
	c := turn.NewConversation(&esphome.VoiceSatellite{ActiveWakeWords: []string{"B"}}, turn.Options{Word: selected.ID})
	if phrase, ok := c.PhraseFor(1); !ok || phrase != "B" {
		t.Fatal(phrase, ok)
	}
	if _, ok := c.PhraseFor(0); ok {
		t.Fatal("empty primary has a phrase")
	}
}
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
	if got := chosen(wanted(installed, wakeword.Slots)); len(got) != 0 {
		t.Fatal("disabled selection re-enabled", got)
	}
}
