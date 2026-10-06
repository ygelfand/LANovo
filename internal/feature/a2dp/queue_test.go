package a2dp

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/bluetooth/avrcp"
)

// The now playing list arrives whole, current track included, and the card shows what is behind it.

func list() []avrcp.Item {
	return []avrcp.Item{
		{UID: 11, Track: avrcp.Track{Title: "Black Cow"}},
		{UID: 12, Track: avrcp.Track{Title: "Aja"}},
		{UID: 13, Track: avrcp.Track{Title: "Deacon Blues"}},
	}
}

func titles(items []avrcp.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Track.Title)
	}
	return out
}

func same(t *testing.T, got []avrcp.Item, want ...string) {
	t.Helper()

	names := titles(got)
	if len(names) != len(want) {
		t.Fatalf("up next is %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("up next is %v, want %v", names, want)
		}
	}
}

// A uid is a position, not a name: the same few come round as the window moves. A handle learned
// for one listing names whatever is at that position in the next.
func TestANewListingForgetsTheOldHandles(t *testing.T) {
	s := &Sink{}

	s.holds(list())
	s.mu.Lock()
	s.handles = map[uint64]string{11: "2964601"}
	s.mu.Unlock()

	s.holds(list())

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.handles) != 0 {
		t.Errorf("a handle survived the listing it was learned for: %v", s.handles)
	}
}

func TestTheUidSaysWhereInTheListWeAre(t *testing.T) {
	state := avrcp.State{UID: 12, Track: avrcp.Track{Title: "Aja"}}
	same(t, after(list(), state), "Deacon Blues")
}

// A phone that keeps no list of its own sends no uid, and the title is what is left to match on.
func TestWithoutAUidTheTitleIsWhatMatches(t *testing.T) {
	state := avrcp.State{Track: avrcp.Track{Title: "Aja"}}
	same(t, after(list(), state), "Deacon Blues")
}

// The uid a phone sends when it has stopped is all ones, which is not an entry.
func TestNothingPlayingLeavesTheWholeList(t *testing.T) {
	state := avrcp.State{UID: nothing}
	same(t, after(list(), state), "Black Cow", "Aja", "Deacon Blues")
}

// A track that is not in the list is a list that has not started, not an empty one.
func TestATrackThatIsNotInTheListLeavesItWhole(t *testing.T) {
	state := avrcp.State{UID: 99, Track: avrcp.Track{Title: "Peg"}}
	same(t, after(list(), state), "Black Cow", "Aja", "Deacon Blues")
}

// The last entry playing means there is nothing behind it.
func TestTheEndOfTheListIsEmpty(t *testing.T) {
	state := avrcp.State{UID: 13, Track: avrcp.Track{Title: "Deacon Blues"}}
	same(t, after(list(), state))
}
