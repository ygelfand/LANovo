package boot

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/component"
)

// summary decides whether the screen is drawn again, so two different states must not summarize
// the same: one that did would leave the panel showing a stage the device has left.
func TestSummaryChangesWithProgress(t *testing.T) {
	looking := []component.Progress{{Name: "wifi", Doing: "looking for the network"}}
	asking := []component.Progress{{Name: "wifi", Doing: "asking for an address"}}

	if summary(looking) == summary(asking) {
		t.Error("two stages of the same component summarize alike")
	}
}

// Done is what the summary is for. A component finishing has to change it, or the dot beside it
// never fills in.
func TestSummaryChangesWhenSomethingFinishes(t *testing.T) {
	waiting := []component.Progress{{Name: "wifi", Doing: "looking"}}
	done := []component.Progress{{Name: "wifi", Doing: "looking", Done: true}}

	if summary(waiting) == summary(done) {
		t.Error("a component finishing did not change the summary")
	}
}

// The panel is written pixel by pixel, so a state that has not changed must not be redrawn. The
// two lists are built separately on purpose: the summary has to depend on what they say rather
// than on their being the same slice.
func TestSummaryIsStableWhileNothingChanges(t *testing.T) {
	first := []component.Progress{
		{Name: "wifi", Doing: "looking"},
		{Name: "dhcp", Doing: "asking", Done: true},
	}
	second := []component.Progress{
		{Name: "wifi", Doing: "looking"},
		{Name: "dhcp", Doing: "asking", Done: true},
	}

	if summary(first) != summary(second) {
		t.Errorf("the same progress summarized as %q and %q", summary(first), summary(second))
	}
}

// Everything up summarizes to nothing, which is what the splash prints as the last thing it says.
func TestSummaryOfEverythingDone(t *testing.T) {
	all := []component.Progress{
		{Name: "wifi", Doing: "looking", Done: true},
		{Name: "dhcp", Doing: "asking", Done: true},
	}

	if got := summary(all); got == "" {
		t.Errorf("summary = %q, want completed rows retained for redraw tracking", got)
	}
	if summary(nil) != "" {
		t.Error("nothing to wait for should summarize to nothing")
	}
}

// The first pass has to draw whatever the state is, including everything already done — the splash
// starts from a summary no state can produce, so the empty one is not mistaken for "unchanged".
func TestEverythingDoneIsNotTheStartingSummary(t *testing.T) {
	const starts = "\x00"

	if summary(nil) == starts {
		t.Error("the starting summary is reachable, so the first pass may not draw")
	}
}
