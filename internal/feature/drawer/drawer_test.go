package drawer

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

func called(s string) func() string { return func() string { return s } }

func TestEdgesRoundTripToTheTouchscreen(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range []config.Edge{config.EdgeLeft, config.EdgeRight, config.EdgeTop, config.EdgeBottom} {
		got := asTouch(e).String()
		if seen[got] {
			t.Errorf("two edges both map to %q", got)
		}
		seen[got] = true
	}
}

func TestTheOrderIsDeclaredNotTheOrderAdded(t *testing.T) {
	r := &Rail{}

	for _, e := range []Entry{
		{Name: called("Idle"), Order: OrderIdle},
		{Name: called("Player"), Order: OrderPlayer},
		{Name: called("Settings"), Order: OrderSettings},
	} {
		r.Add(e)
	}

	want := []string{"Settings", "Player", "Idle"}
	for i, e := range r.Entries() {
		if e.Label() != want[i] {
			t.Errorf("position %d is %s, want %s", i, e.Label(), want[i])
		}
	}
}

func TestEqualOrdersFallBackToTheName(t *testing.T) {
	first, second := &Rail{}, &Rail{}

	first.Add(Entry{Name: called("Zebra"), Order: 50})
	first.Add(Entry{Name: called("Aardvark"), Order: 50})

	second.Add(Entry{Name: called("Aardvark"), Order: 50})
	second.Add(Entry{Name: called("Zebra"), Order: 50})

	for i, e := range first.Entries() {
		if got := second.Entries()[i].Label(); got != e.Label() {
			t.Errorf("position %d is %s one way round and %s the other", i, e.Label(), got)
		}
	}
	if first.Entries()[0].Label() != "Aardvark" {
		t.Errorf("the tiebreak is not the name: %s came first", first.Entries()[0].Label())
	}
}

func TestTheDeclaredPlacesAreDistinct(t *testing.T) {
	seen := map[int]bool{}

	for _, at := range []int{OrderSettings, OrderPlayer, OrderIdle} {
		if seen[at] {
			t.Errorf("two things are declared at %d", at)
		}
		seen[at] = true
	}
}
