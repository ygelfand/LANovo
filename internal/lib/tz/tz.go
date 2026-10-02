// Package tz puts the device in a time zone named by a POSIX TZ string.
//
// Home Assistant sends one — EST5EDT,M3.2.0,M11.1.0 — rather than a zone name, because that is
// what an ESPHome device consumes: rules, not a database to look them up in.
//
// Go reads those, in tzset, but only as the footer of a compiled zone file. So it is given one: a
// zone file holding a single transition at the dawn of time, with the string after it. Everything
// from that transition onwards, which is everything, is then decided by the rules.
//
// The alternative was to work the transitions out here, which is what ESPHome does in posix_tz.cpp.
// It is not arithmetic worth owning a second copy of.
package tz

import (
	"encoding/binary"
	"fmt"
	"time"
)

// Use puts the process in the zone spec describes. An empty spec leaves it where it is.
func Use(spec string) error {
	if spec == "" {
		return nil
	}

	loc, err := Location(spec)
	if err != nil {
		return err
	}

	time.Local = loc
	return nil
}

// Location is the zone spec describes.
//
// A footer it cannot parse is not an error to LoadLocationFromTZData — it drops the rules and
// leaves the transitions, which here are the one placeholder. So the answer is checked: anything
// still reading as the placeholder means the string never parsed, and silently keeping UTC would
// look like a clock that lost an hour for no reason.
func Location(spec string) (*time.Location, error) {
	loc, err := time.LoadLocationFromTZData(spec, tzif(spec))
	if err != nil {
		return nil, fmt.Errorf("tz: %q: %w", spec, err)
	}
	if zone, _ := time.Now().In(loc).Zone(); zone == placeholder {
		return nil, fmt.Errorf("tz: %q: not a POSIX TZ string", spec)
	}
	return loc, nil
}

// Current is the string the device is keeping time by, empty for UTC.
func Current() string {
	if time.Local == time.UTC {
		return ""
	}
	return time.Local.String()
}

// What the zone file holds: one transition, one type, one name.
const (
	magic   = "TZif"
	version = '2'

	changes = 1
	types   = 1

	// placeholder names the one type in the file. Nothing reads it when the footer parses, so it
	// is a name no TZ string produces: seeing it back is how a footer that did not parse is caught.
	placeholder = "<>"
	name        = placeholder + "\x00"
)

// tzif is a compiled zone file carrying nothing but spec.
//
// Version 2: a 32-bit block, the same again in 64 bits, then the string. Go needs a transition to
// hang the string off — it reaches for the rules once it runs out of transitions — so there is
// exactly one, as early as the block can express.
func tzif(spec string) []byte {
	var b []byte

	header := func() {
		b = append(b, magic...)
		b = append(b, version)
		b = append(b, make([]byte, 15)...) // reserved

		// isutcnt, isstdcnt, leapcnt, timecnt, typecnt, charcnt
		for _, n := range []uint32{0, 0, 0, changes, types, uint32(len(name))} {
			b = binary.BigEndian.AppendUint32(b, n)
		}
	}

	block := func(wide bool) {
		if wide {
			b = binary.BigEndian.AppendUint64(b, 1<<63)
		} else {
			b = binary.BigEndian.AppendUint32(b, 1<<31)
		}
		b = append(b, 0) // the type that transition selects

		// The type itself: no offset, not daylight saving, named by the string below. What it says
		// does not matter — every time anyone asks about is past the transition, and answered by
		// the rules.
		b = binary.BigEndian.AppendUint32(b, 0)
		b = append(b, 0, 0)

		b = append(b, name...)
	}

	header()
	block(false)
	header()
	block(true)

	return append(append(append(b, '\n'), spec...), '\n')
}
