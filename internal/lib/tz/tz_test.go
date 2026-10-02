package tz

import (
	"testing"
	"time"
)

// The whole point of synthesizing a zone file: the rules in the string have to survive into the
// Location, so a time on either side of a transition gets the right offset without a database.
func TestDaylightSavingComesFromTheRules(t *testing.T) {
	tests := []struct {
		name string
		spec string
		at   time.Time
		zone string
		west int // hours behind UTC, negative for ahead
	}{
		{"new york in winter", "EST5EDT,M3.2.0,M11.1.0", time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC), "EST", 5},
		{"new york in summer", "EST5EDT,M3.2.0,M11.1.0", time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), "EDT", 4},
		{"the hour it springs forward", "EST5EDT,M3.2.0,M11.1.0", time.Date(2026, 3, 8, 7, 30, 0, 0, time.UTC), "EDT", 4},
		{"the hour before", "EST5EDT,M3.2.0,M11.1.0", time.Date(2026, 3, 8, 6, 30, 0, 0, time.UTC), "EST", 5},
		{"london in winter", "GMT0BST,M3.5.0/1,M10.5.0", time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC), "GMT", 0},
		{"london in summer", "GMT0BST,M3.5.0/1,M10.5.0", time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), "BST", -1},
		{"sydney, south of the equator", "AEST-10AEDT,M10.1.0,M4.1.0/3", time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC), "AEDT", -11},
		{"sydney in its winter", "AEST-10AEDT,M10.1.0,M4.1.0/3", time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), "AEST", -10},
		{"a zone that never changes", "MST7", time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), "MST", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc, err := Location(tt.spec)
			if err != nil {
				t.Fatalf("Location(%q): %v", tt.spec, err)
			}

			zone, offset := tt.at.In(loc).Zone()
			if zone != tt.zone {
				t.Errorf("zone = %q, want %q", zone, tt.zone)
			}
			if offset != -tt.west*3600 {
				t.Errorf("offset = %ds, want %ds", offset, -tt.west*3600)
			}
		})
	}
}

// Minutes in the offset, which a device west of a whole hour needs.
func TestOffsetsThatAreNotWholeHours(t *testing.T) {
	loc, err := Location("IST-5:30")
	if err != nil {
		t.Fatalf("Location: %v", err)
	}

	_, offset := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC).In(loc).Zone()
	if want := 5*3600 + 30*60; offset != want {
		t.Errorf("offset = %ds, want %ds", offset, want)
	}
}

// Every instant is past the one synthesized transition, so the rules answer for all of them. A
// date before the Unix epoch is the case that would fall through to the placeholder type.
func TestTheRulesAnswerBeforeTheEpochToo(t *testing.T) {
	loc, err := Location("EST5EDT,M3.2.0,M11.1.0")
	if err != nil {
		t.Fatalf("Location: %v", err)
	}

	zone, offset := time.Date(1965, 7, 15, 12, 0, 0, 0, time.UTC).In(loc).Zone()
	if zone != "EDT" || offset != -4*3600 {
		t.Errorf("1965 resolved to %s %ds, want EDT -14400", zone, offset)
	}
}

// A string that is not a TZ string leaves the device where it was, rather than throwing it to UTC,
// which would look like a clock that lost an hour for no reason.
func TestNonsenseIsRefused(t *testing.T) {
	was := time.Local
	t.Cleanup(func() { time.Local = was })

	if err := Use("EST5EDT,M3.2.0,M11.1.0"); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if err := Use("not a time zone at all"); err == nil {
		t.Error("nonsense was accepted")
	}
	if got := Current(); got != "EST5EDT,M3.2.0,M11.1.0" {
		t.Errorf("zone = %q, want the one that worked", got)
	}
}

// Nothing to set is not an error: a device that has never been told stays where it is.
func TestAnEmptyZoneIsNotAnError(t *testing.T) {
	was := time.Local
	t.Cleanup(func() { time.Local = was })

	if err := Use(""); err != nil {
		t.Errorf("Use(\"\"): %v", err)
	}
	if time.Local != was {
		t.Error("an empty zone moved the device")
	}
}

// Current names the device by the string it was given, which is what is written to the config and
// compared against the next one Home Assistant sends.
func TestCurrentIsTheStringItWasGiven(t *testing.T) {
	was := time.Local
	t.Cleanup(func() { time.Local = was })

	const spec = "CET-1CEST,M3.5.0,M10.5.0/3"
	if err := Use(spec); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if got := Current(); got != spec {
		t.Errorf("Current() = %q, want %q", got, spec)
	}
}
