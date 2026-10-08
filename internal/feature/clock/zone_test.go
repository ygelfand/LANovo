package clock

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	tz "github.com/ygelfand/libcountertop/pkg/timezone"
)

func TestAChosenZoneOutranksTheServer(t *testing.T) {
	newYork, ok := tz.Named("America/New_York")
	if !ok {
		t.Fatal("America/New_York is not offered")
	}

	got := spec(schema.Time{
		Home:   "GMT0BST,M3.5.0/1,M10.5.0",
		Chosen: newYork.Name,
	})
	if got != newYork.Spec {
		t.Errorf("the device would run on %q, want the chosen %q", got, newYork.Spec)
	}
}

func TestWithNothingChosenTheServerDecides(t *testing.T) {
	const said = "GMT0BST,M3.5.0/1,M10.5.0"

	if got := spec(schema.Time{Home: said}); got != said {
		t.Errorf("the device would run on %q, want the server's %q", got, said)
	}
}

func TestGivingTheChoiceBackFallsToWhatTheServerSaid(t *testing.T) {
	const said = "GMT0BST,M3.5.0/1,M10.5.0"

	was := schema.Time{Home: said, Chosen: "Asia/Tokyo"}
	if got := spec(was); got == said {
		t.Fatal("the chosen zone was not being used, so this proves nothing")
	}

	was.Chosen = ""
	if got := spec(was); got != said {
		t.Errorf("after giving the choice back the device runs on %q, want %q", got, said)
	}
}

func TestAnUnknownChoiceFallsBackRatherThanBreaking(t *testing.T) {
	const said = "EST5EDT,M3.2.0,M11.1.0"

	if got := spec(schema.Time{Home: said, Chosen: "Nowhere/Special"}); got != said {
		t.Errorf("an unknown choice gave %q, want the server's %q", got, said)
	}
}

func TestNothingKnownIsNoZone(t *testing.T) {
	if got := spec(schema.Time{}); got != "" {
		t.Errorf("with nothing known the device would run on %q", got)
	}
}

func TestTheSelectOffersEverywhereAndTheWayBack(t *testing.T) {
	c := &Clock{}
	c.buildZone()

	if got, want := len(c.zone.Options), len(tz.Names())+1; got != want {
		t.Errorf("%d options, want %d zones and Home Assistant", got, want)
	}
	if c.zone.Options[0] != FollowHome {
		t.Errorf("the first option is %q, want %q", c.zone.Options[0], FollowHome)
	}
	if _, ok := tz.Named(FollowHome); ok {
		t.Errorf("%q is also a zone name, so choosing it is ambiguous", FollowHome)
	}
}

func TestTheLabelSaysWhoIsDeciding(t *testing.T) {
	if got := label(""); got != FollowHome {
		t.Errorf("with nothing chosen the label is %q, want %q", got, FollowHome)
	}
	if got := label("Asia/Tokyo"); got != "Asia/Tokyo" {
		t.Errorf("a chosen zone reads as %q", got)
	}
}
