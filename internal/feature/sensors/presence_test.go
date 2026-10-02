package sensors

import (
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
)

func feed(p *presence, at *time.Time, v float64, n int) (near, changed bool) {
	for range n {
		*at = at.Add(interval)
		var c bool
		near, c = p.update(v, rise(config.DefaultPresenceRange), *at)
		changed = changed || c
	}
	return near, changed
}

func TestNoiseAtTheBaselineIsNobody(t *testing.T) {
	var p presence
	at := time.Unix(0, 0)
	for i := range 200 {
		if near, _ := feed(&p, &at, float64(4+i%3), 1); near {
			t.Fatalf("reading %d at the glass's own level counted as somebody", i)
		}
	}
}

func TestApproachArrivesAndLeavingWaitsForTheHold(t *testing.T) {
	var p presence
	at := time.Unix(0, 0)
	feed(&p, &at, 5, 20)

	if near, _ := feed(&p, &at, 30, 1); near {
		t.Fatal("one reading above the baseline counted as somebody")
	}
	if near, changed := feed(&p, &at, 30, 1); !near || !changed {
		t.Fatal("two readings above the baseline did not count as somebody")
	}

	if near, _ := feed(&p, &at, 5, int(nearHold/interval)-1); !near {
		t.Fatal("somebody left before the hold ran out")
	}
	if near, _ := feed(&p, &at, 30, 1); !near {
		t.Fatal("coming back within the hold did not keep them present")
	}
	if near, changed := feed(&p, &at, 5, int(nearHold/interval)+1); near || !changed {
		t.Fatal("nobody was reported once the hold ran out")
	}
}

func TestSomebodyStayingDoesNotRaiseTheBaseline(t *testing.T) {
	var p presence
	at := time.Unix(0, 0)
	feed(&p, &at, 5, 20)
	feed(&p, &at, 30, int(10*time.Minute/interval))
	if p.base > 6 {
		t.Fatalf("baseline rose to %.1f while somebody stood there", p.base)
	}
}

func TestSomethingLeftInFrontIsRelearned(t *testing.T) {
	var p presence
	at := time.Unix(0, 0)
	feed(&p, &at, 5, 20)
	feed(&p, &at, 30, int(nearStuck/interval)+2)
	if near, _ := feed(&p, &at, 30, 4); near {
		t.Fatal("an object parked in front still counts as somebody after an hour")
	}
}
