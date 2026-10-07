package media

import (
	"testing"
)

// Who the card belongs to.
//
// Four things can be playing — a url Home Assistant sent, a sendspin group, a phone over Bluetooth,
// something casting — and there is one card and one media entity for all of them. The rule is that
// the last one to start playing holds the card and goes on holding it once it stops.
//
// They are peers. None of them is the special case the others are measured against, which is what
// these check: whichever plays takes it, whoever has gone takes back only their own.

// peer stands in for any of them: something that plays and says what it is playing. Held by
// pointer, the way the real ones are, so releasing compares what it is rather than what it holds.
type peer struct {
	now  Now
	kind Kind
}

func (*peer) Play()         {}
func (*peer) Pause()        {}
func (*peer) Stop()         {}
func (*peer) Next()         {}
func (*peer) Previous()     {}
func (p *peer) Now() Now    { return p.now }
func (p *peer) Kind() Kind  { return p.kind }
func (*peer) Label() string { return "the kitchen" }

// held is what the card currently reads.
func held(t *testing.T) Now {
	t.Helper()
	return Get().Now()
}

// The case that was broken: a phone connecting over Bluetooth while a group played took the card,
// answered with nothing because it was playing nothing, and never gave it back.
func TestTheLastThingToPlayHoldsTheCard(t *testing.T) {
	p := Get()
	t.Cleanup(func() { p.External(nil) })

	group := &peer{now: Now{Playing: true, Title: "Silent Hill", Can: CanPause}}
	phone := &peer{now: Now{Playing: true, Title: "Aja"}}

	p.External(group)
	if got := held(t); got.Title != "Silent Hill" {
		t.Fatalf("the group played and the card reads %q", got.Title)
	}

	p.External(phone)
	if got := held(t); got.Title != "Aja" {
		t.Errorf("the phone played and the card reads %q", got.Title)
	}

	p.External(group)
	if got := held(t); got.Title != "Silent Hill" {
		t.Errorf("the group played again and the card reads %q", got.Title)
	}
}

// Stopping does not give the card up: what was playing a moment ago is still the answer to what
// this screen is about, and the transport has to keep reaching it.
func TestStoppingKeepsTheCard(t *testing.T) {
	p := Get()
	t.Cleanup(func() { p.External(nil) })

	played := &peer{now: Now{Playing: true, Title: "Aja", Can: CanPause}}
	p.External(played)

	played.now = Now{Paused: true, Title: "Aja", Can: CanPause}
	p.External(played)

	if got := held(t); got.Title != "Aja" || !got.Paused {
		t.Errorf("a paused holder reads %+v, want it still holding the card", got)
	}
}

// A source that has gone must not blank whoever took over since.
func TestReleasingIsOnlyEverYourOwn(t *testing.T) {
	p := Get()
	t.Cleanup(func() { p.External(nil) })

	holder := &peer{now: Now{Playing: true, Title: "Aja"}}
	other := &peer{now: Now{Playing: true, Title: "Something Else"}}

	p.External(holder)
	p.Release(other)

	if got := held(t); got.Title != "Aja" {
		t.Errorf("releasing another source blanked the holder: %q", got.Title)
	}

	p.Release(holder)
	if got := p.source(); got != nil {
		t.Errorf("the holder released its own card and still holds it: %+v", got.Now())
	}
}

// Home Assistant is a peer rather than the thing the others are measured against. A url playing
// takes the card from a group, which is the fault that the arbitration this replaced was written
// for: the group's old track used to sit on the screen while a url played.
func TestAUrlTakesTheCardLikeAnythingElse(t *testing.T) {
	p := Get()
	t.Cleanup(func() { p.External(nil) })

	p.External(&peer{now: Now{Playing: true, Title: "Something Earlier"}})

	p.External(homeAssistant{})
	if got := held(t); got.Title != "" {
		t.Errorf("a url is playing and the card reads %q, want no metadata", got.Title)
	}
	if got := p.source(); got == nil {
		t.Error("a url playing holds no card")
	}
}

// Before anything has played there is no holder, and the card is whatever the queue says.
func TestNothingHasPlayedYet(t *testing.T) {
	p := Get()
	p.External(nil)

	if got := p.source(); got != nil {
		t.Errorf("something holds the card before anything played: %+v", got.Now())
	}
	if got := held(t); got.Title != "" || got.Playing {
		t.Errorf("an idle player reads %+v", got)
	}
}

type stopping struct {
	peer
	stopped int
}

func (s *stopping) Stop() { s.stopped++ }

func TestAReplacedSourceIsStoppedForGood(t *testing.T) {
	p := Get()
	group, cast, next := &stopping{peer: peer{kind: FromGroup}}, &peer{kind: FromCast}, &peer{kind: FromCast}
	t.Cleanup(func() { p.Ended(group); p.Ended(cast); p.External(nil) })

	p.Began(group)
	p.Began(group)
	p.Began(cast)
	p.Began(next)
	if group.stopped != 1 {
		t.Errorf("the group was stopped %d times when a cast replaced it, want once", group.stopped)
	}
	if p.source() != Source(next) {
		t.Errorf("the card is %v, want the cast's next track", p.source())
	}

	p.Ended(next)
	p.Release(next)
	if p.source() != nil {
		t.Errorf("after the cast left the card went to %v, want nobody", p.source())
	}
}

func TestOnlyANewSessionOpensThePlayer(t *testing.T) {
	p := Get()
	requests := 0
	stop := p.Begun.Listen(func(Source) { requests++ })
	defer stop()
	opens := func() bool { n := requests; requests = 0; return n > 0 }
	phone, group := &peer{kind: FromBluetooth}, &peer{kind: FromGroup}
	t.Cleanup(func() { p.Ended(phone); p.Ended(group); p.External(nil) })
	opens()

	p.External(phone)
	if opens() {
		t.Error("claiming the card without beginning opened the player")
	}

	p.Began(phone)
	if !opens() {
		t.Error("a source beginning did not open the player")
	}

	p.Began(phone)
	if opens() {
		t.Error("the same source beginning again, a next track or a resume, opened the player")
	}

	p.Began(&peer{kind: FromBluetooth})
	if opens() {
		t.Error("another source of the same kind, a cast's next track, opened the player")
	}
	p.Began(phone)
	opens()

	p.Began(group)
	if !opens() {
		t.Error("another source beginning did not open the player")
	}

	p.Began(phone)
	if !opens() {
		t.Error("coming back after another source took over did not open the player")
	}

	p.Ended(phone)
	p.Began(phone)
	if !opens() {
		t.Error("beginning after its session ended did not open the player")
	}
}
