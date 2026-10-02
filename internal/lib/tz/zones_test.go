package tz

import (
	"testing"
	"time"
)

// Every offered zone is a POSIX TZ string written out by hand, so every one is a chance to get a
// daylight saving rule wrong. This walks a year against the host's own tzdata and fails on any
// hour that disagrees, which is what catches a transition on the wrong Sunday.
//
// The host has zone files; the device does not, which is the whole reason the rules are carried
// rather than looked up. So this checks the data at the only point anything authoritative is to
// hand.
func TestEveryZoneMatchesTheRealOne(t *testing.T) {
	for _, z := range Zones() {
		real, err := time.LoadLocation(z.Name)
		if err != nil {
			t.Skipf("no zone files on this machine: %v", err)
		}

		ours, err := Location(z.Spec)
		if err != nil {
			t.Errorf("%s: %q does not parse: %v", z.Name, z.Spec, err)
			continue
		}

		// Hourly through a year, which lands on both transitions and either side of each.
		from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		bad := 0

		for at := from; at.Before(from.AddDate(1, 0, 0)); at = at.Add(time.Hour) {
			_, want := at.In(real).Zone()
			_, got := at.In(ours).Zone()

			if got == want {
				continue
			}
			bad++
			if bad == 1 {
				t.Errorf("%s: at %s the offset is %d, want %d (%q)",
					z.Name, at.Format(time.RFC3339), got, want, z.Spec)
			}
		}
		if bad > 1 {
			t.Errorf("%s: %d hours of the year disagree", z.Name, bad)
		}
	}
}

func TestEveryZoneParses(t *testing.T) {
	for _, z := range Zones() {
		if _, err := Location(z.Spec); err != nil {
			t.Errorf("%s: %v", z.Name, err)
		}
	}
}

// The names are what a chosen zone is stored as, so two of them the same would make one
// unreachable.
func TestTheNamesAreDistinct(t *testing.T) {
	seen := map[string]bool{}

	for _, z := range Zones() {
		if seen[z.Name] {
			t.Errorf("%s is offered twice", z.Name)
		}
		seen[z.Name] = true
	}
}

func TestNamedFindsWhatIsOffered(t *testing.T) {
	for _, want := range Zones() {
		got, ok := Named(want.Name)
		if !ok {
			t.Errorf("%s is offered but cannot be looked up", want.Name)
			continue
		}
		if got.Spec != want.Spec {
			t.Errorf("%s resolved to %q, want %q", want.Name, got.Spec, want.Spec)
		}
	}

	if _, ok := Named("Nowhere/Special"); ok {
		t.Error("a zone that is not offered was found")
	}
}

// Every region has to fit on a page, which does not scroll. The longest one decides.
func TestNoRegionOverflowsAPage(t *testing.T) {
	// About what the body of a settings page holds before rows run off the bottom.
	const fits = 13

	for _, r := range Regions() {
		if n := len(In(r)); n > fits {
			t.Errorf("%s has %d zones, more than the %d a page shows", r, n, fits)
		}
	}

	// The first page is the way out, the zones with no region, and one row per region.
	if n := 1 + len(In("")) + len(Regions()); n > fits {
		t.Errorf("the first page has %d rows, more than the %d it shows", n, fits)
	}
}

// Every zone has to be reachable: one with no region shows on the first page, one with a region
// shows under it, and nothing falls between.
func TestEveryZoneIsReachable(t *testing.T) {
	seen := map[string]bool{}

	for _, r := range append([]string{""}, Regions()...) {
		for _, z := range In(r) {
			if seen[z.Name] {
				t.Errorf("%s is offered in more than one region", z.Name)
			}
			seen[z.Name] = true
		}
	}

	for _, z := range Zones() {
		if !seen[z.Name] {
			t.Errorf("%s is in no region and not on the first page", z.Name)
		}
	}
}
