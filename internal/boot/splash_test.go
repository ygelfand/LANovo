package boot

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/component"
)

func TestSummaryChangesWithProgress(t *testing.T) {
	looking := []component.Progress{{Name: "wifi", Doing: "looking for the network"}}
	asking := []component.Progress{{Name: "wifi", Doing: "asking for an address"}}

	if summary(looking) == summary(asking) {
		t.Error("two stages of the same component summarize alike")
	}
}

func TestSummaryChangesWhenSomethingFinishes(t *testing.T) {
	waiting := []component.Progress{{Name: "wifi", Doing: "looking"}}
	done := []component.Progress{{Name: "wifi", Doing: "looking", Done: true}}

	if summary(waiting) == summary(done) {
		t.Error("a component finishing did not change the summary")
	}
}

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

func TestEverythingDoneIsNotTheStartingSummary(t *testing.T) {
	const starts = "\x00"

	if summary(nil) == starts {
		t.Error("the starting summary is reachable, so the first pass may not draw")
	}
}
