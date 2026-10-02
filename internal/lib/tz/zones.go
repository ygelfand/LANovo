package tz

// Zone is somewhere to keep time: the name people recognize, and the rules the device runs on.
type Zone struct {
	// Name is the IANA name, which is the only part anybody wants to read.
	Name string

	// Spec is the POSIX TZ string. This device has no zone files to look a name up in — working
	// from rules instead is what this package is for — so the rules are carried here.
	Spec string
}

// Zones is what the device offers to be set to.
//
// Deliberately a short list and not a database. Every entry is written out by hand, so every entry
// is a chance to get a daylight saving rule wrong; TestEveryZoneMatchesTheRealOne checks each one
// against the host's own tzdata across a year and fails on any day that disagrees. Adding a zone
// means adding the pair and letting the test say whether the rules are right.
//
// Enough of the world to cover where one of these is likely to sit. Anything missing is a pair
// away, and the alternative — every zone there is — would be a megabyte of tables for a setting
// most devices never touch, when Home Assistant already sends the right answer for almost all of
// them.
//
// Not every zone can be offered. America/Vancouver is left out because tzdata has British Columbia
// going to permanent daylight time part way through 2026, modeled as a move to MST; a POSIX rule
// repeats every year and cannot say "and then something else from November". Any zone the test
// rejects for that reason belongs out of this list rather than approximated into it.
func Zones() []Zone {
	return []Zone{
		{"UTC", "UTC0"},

		{"Europe/London", "GMT0BST,M3.5.0/1,M10.5.0"},
		{"Europe/Lisbon", "WET0WEST,M3.5.0/1,M10.5.0"},
		{"Europe/Paris", "CET-1CEST,M3.5.0,M10.5.0/3"},
		{"Europe/Berlin", "CET-1CEST,M3.5.0,M10.5.0/3"},
		{"Europe/Madrid", "CET-1CEST,M3.5.0,M10.5.0/3"},
		{"Europe/Rome", "CET-1CEST,M3.5.0,M10.5.0/3"},
		{"Europe/Amsterdam", "CET-1CEST,M3.5.0,M10.5.0/3"},
		{"Europe/Warsaw", "CET-1CEST,M3.5.0,M10.5.0/3"},
		{"Europe/Athens", "EET-2EEST,M3.5.0/3,M10.5.0/4"},
		{"Europe/Helsinki", "EET-2EEST,M3.5.0/3,M10.5.0/4"},
		{"Europe/Moscow", "MSK-3"},

		{"America/New_York", "EST5EDT,M3.2.0,M11.1.0"},
		{"America/Chicago", "CST6CDT,M3.2.0,M11.1.0"},
		{"America/Denver", "MST7MDT,M3.2.0,M11.1.0"},
		{"America/Phoenix", "MST7"},
		{"America/Los_Angeles", "PST8PDT,M3.2.0,M11.1.0"},
		{"America/Anchorage", "AKST9AKDT,M3.2.0,M11.1.0"},
		{"Pacific/Honolulu", "HST10"},
		{"America/Toronto", "EST5EDT,M3.2.0,M11.1.0"},
		{"America/Mexico_City", "CST6"},
		{"America/Sao_Paulo", "<-03>3"},

		{"Asia/Jerusalem", "IST-2IDT,M3.4.4/26,M10.5.0"},
		{"Asia/Dubai", "<+04>-4"},
		{"Asia/Kolkata", "IST-5:30"},
		{"Asia/Bangkok", "<+07>-7"},
		{"Asia/Shanghai", "CST-8"},
		{"Asia/Singapore", "<+08>-8"},
		{"Asia/Hong_Kong", "HKT-8"},
		{"Asia/Tokyo", "JST-9"},
		{"Asia/Seoul", "KST-9"},

		{"Australia/Perth", "AWST-8"},
		{"Australia/Brisbane", "AEST-10"},
		{"Australia/Sydney", "AEST-10AEDT,M10.1.0,M4.1.0/3"},
		{"Australia/Adelaide", "ACST-9:30ACDT,M10.1.0,M4.1.0/3"},
		{"Pacific/Auckland", "NZST-12NZDT,M9.5.0,M4.1.0/3"},

		{"Africa/Johannesburg", "SAST-2"},
		{"Africa/Lagos", "WAT-1"},
		{"Africa/Cairo", "EET-2EEST,M4.5.5/0,M10.5.4/24"},
	}
}

// Region is the part of the world a zone is in, taken from its name. Empty for a name with no
// region in it, which is UTC and anything else that stands on its own.
//
// Derived rather than carried in a field: the name already says which is which, and a second
// source of the same fact is a second thing to get wrong.
func Region(name string) string {
	for i := range name {
		if name[i] == '/' {
			return name[:i]
		}
	}
	return ""
}

// Regions is every part of the world with zones in it, first seen first. A screen holds about a
// dozen rows and does not scroll, so the list is offered a region at a time.
func Regions() []string {
	var out []string
	seen := map[string]bool{}

	for _, z := range Zones() {
		r := Region(z.Name)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

// In is every zone in one region. The empty region is the ones that stand on their own.
func In(region string) []Zone {
	var out []Zone

	for _, z := range Zones() {
		if Region(z.Name) == region {
			out = append(out, z)
		}
	}
	return out
}

// Named is the zone with this IANA name.
func Named(name string) (Zone, bool) {
	for _, z := range Zones() {
		if z.Name == name {
			return z, true
		}
	}
	return Zone{}, false
}

// Names is every zone on offer, in the order Zones lists them.
func Names() []string {
	zones := Zones()

	out := make([]string, 0, len(zones))
	for _, z := range zones {
		out = append(out, z.Name)
	}
	return out
}
