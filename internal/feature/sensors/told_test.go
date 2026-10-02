package sensors

import (
	"testing"
	"time"
)

// The numbers are room lux, which is what the sensor reports once the glass over it is accounted
// for: a lit room measured against a phone light meter at about 190.
const lit = 190.0

func TestFirstReadingIsAlwaysSent(t *testing.T) {
	var sent told

	if !sent.worth(lit, leastLux) {
		t.Fatal("the first reading was held back, so Home Assistant would start with nothing")
	}
}

func TestAReadingThatHasNotMovedIsHeldBack(t *testing.T) {
	var sent told
	sent.worth(lit, leastLux)

	// What the part does at rest: about twelve counts end to end, which is ten lux of room.
	for _, lux := range []float64{190, 196, 184, 193, 187} {
		if sent.worth(lux, leastLux) {
			t.Errorf("%.0f lux was reported, which is inside the part's own noise", lux)
		}
	}
}

func TestAReadingThatHasMovedIsSent(t *testing.T) {
	var sent told
	sent.worth(lit, leastLux)

	// A lamp switched on: well past five percent of where it was.
	if !sent.worth(380, leastLux) {
		t.Fatal("the room doubling was not reported")
	}
}

// The floor is what stops a dark room reporting on every tick, where five percent is less than the
// part can resolve through the glass.
func TestNearDarkIsHeldToTheFloorRatherThanTheFraction(t *testing.T) {
	var sent told
	sent.worth(20, leastLux)

	if sent.worth(28, leastLux) {
		t.Error("8 lux was reported: five percent of a dark room is under the noise")
	}
	if !sent.worth(40, leastLux) {
		t.Error("20 lux was not reported, which is past the floor")
	}
}

// Comparing against the last reading sent, rather than the last one taken, is what stops a value
// drifting across the threshold from reporting every time it is looked at.
func TestDriftingPastTheThresholdReportsOnce(t *testing.T) {
	var sent told
	sent.worth(400, leastLux)

	// Each step is under five percent of 400, and together they are over it.
	var reports int
	for _, lux := range []float64{408, 416, 424, 432} {
		if sent.worth(lux, leastLux) {
			reports++
		}
	}

	if reports != 1 {
		t.Errorf("a slow drift reported %d times, want once", reports)
	}
}

func TestAStillRoomIsStillHeardFromEventually(t *testing.T) {
	var sent told
	sent.worth(lit, leastLux)

	if sent.worth(lit, leastLux) {
		t.Fatal("an unchanged reading was reported straight away")
	}

	// Long enough that Home Assistant's history would otherwise have a hole in it.
	sent.at = time.Now().Add(-stale - time.Second)

	if !sent.worth(lit, leastLux) {
		t.Error("nothing was reported for an afternoon of an unchanging room")
	}
}
