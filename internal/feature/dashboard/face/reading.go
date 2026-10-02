package face

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Reading is what the clock says at a moment, worked out apart from any drawing so it can be
// checked without a panel.
//
// Every face draws from one of these and none of them reads the clock again. A face that did would
// disagree with the rest across a tick, and the dashboard — which decides whether to repaint by
// comparing readings — would not know anything had changed.
type Reading struct {
	Time   string
	Suffix string
	Date   string

	// Second is the second of the minute, for the one face that shows it. Carried on every reading
	// and left out of String, so a clock that does not have a second hand is not redrawn sixty
	// times a minute for a number nothing draws.
	Second int
}

// Read is what to show at this moment.
//
// The suffix is carried apart from the time so a face can set it smaller and beside it, the way a
// clock is read rather than the way a string formats.
func Read(at time.Time, twentyFour bool) Reading {
	r := Reading{Date: at.Format("Monday, 2 January"), Second: at.Second()}

	if twentyFour {
		r.Time = at.Format("15:04")
		return r
	}

	r.Time = at.Format("3:04")
	r.Suffix = at.Format("PM")
	return r
}

// String is what the dashboard compares to decide whether anything changed, so it has to carry
// everything a face draws from.
func (r Reading) String() string { return fmt.Sprintf("%s%s, %s", r.Time, r.Suffix, r.Date) }

// Undated is the reading with the day dropped, for a device set to show only the time.
//
// Dropped here rather than skipped by each face, so a face has one thing to check and the setting
// cannot be honored by some of them and not others.
func (r Reading) Undated() Reading { r.Date = ""; return r }

// Dated says whether there is a day to draw. A face asks before it reserves room for one: an empty
// string still has a line height, and a face that laid it out anyway would leave a gap under the
// time that nobody can account for.
func (r Reading) Dated() bool { return r.Date != "" }

// lines splits a reading into two, with the hour padded to two digits.
//
// Padded because the faces that stack or box the pair only look right if both are the same width,
// and one digit over two is ragged. A 12 hour clock says 7 on one line and 07 on two, which is what
// every stacked clock does.
func lines(r Reading) (hour, minute string, ok bool) {
	hour, minute, ok = strings.Cut(r.Time, ":")
	if !ok {
		return "", "", false
	}
	if len(hour) == 1 {
		hour = "0" + hour
	}
	return hour, minute, true
}

// clock is the reading as numbers, which is what an angle or a phrase needs and a string cannot
// give. Taken from the same strings the other faces draw, for the reason on Reading.
func clock(r Reading) (hour, minute int, ok bool) {
	h, m, cut := strings.Cut(r.Time, ":")
	if !cut {
		return 0, 0, false
	}

	hour, err := strconv.Atoi(h)
	if err != nil {
		return 0, 0, false
	}

	minute, err = strconv.Atoi(m)
	if err != nil {
		return 0, 0, false
	}
	return hour, minute, true
}
