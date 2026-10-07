package speaker

import (
	"sync"
	"testing"
)

// producer records what it was told. What matters is the state it is left in — a producer left
// standing down never plays again and nothing reports an error about it.
type producer struct {
	mu       sync.Mutex
	down     bool
	ducked   bool
	requeues int
}

func (p *producer) Stand(down bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.down = down
}

func (p *producer) Duck(on bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ducked = on
}

func (p *producer) Requeue() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requeues++
}

func (p *producer) requeued() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.requeues
}

func (p *producer) held() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.down
}

func (p *producer) quiet() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ducked
}

func TestNewestProducerIsTheOneHeard(t *testing.T) {
	a := &Arbiter{}
	group, track := &producer{}, &producer{}

	a.Took(group)
	a.Took(track)

	if !group.held() {
		t.Error("the group kept playing under the track that displaced it")
	}
	if track.held() {
		t.Error("the track that just started was stood down")
	}
	if a.Playing() != track {
		t.Error("the track is not the one being heard")
	}
}

// The point of a stack rather than a single slot: a track is minutes, a noise machine is hours, and
// the room should carry on when the track ends instead of going quiet.
func TestWhatWasInterruptedCarriesOn(t *testing.T) {
	a := &Arbiter{}
	group, track := &producer{}, &producer{}

	a.Took(group)
	a.Took(track)
	a.Gave(track)

	if group.held() {
		t.Error("the group never came back: still standing down with nothing above it")
	}
	if a.Playing() != group {
		t.Error("the group is not the one being heard again")
	}
}

// A producer that finishes while something else is on top was already stood down and is not the one
// being heard, so nothing should be resumed on its account.
func TestFinishingUnderneathChangesNothing(t *testing.T) {
	a := &Arbiter{}
	group, track := &producer{}, &producer{}

	a.Took(group)
	a.Took(track)
	a.Gave(group)

	if track.held() {
		t.Error("the track was stood down when something beneath it finished")
	}
	if a.Playing() != track {
		t.Error("the track stopped being the one heard")
	}
}

// A voice turn holds the speaker across several sounds. Whoever is heard has to stand down once and
// come back once, however the producers change underneath.
func TestTheHoldWinsOverAHandover(t *testing.T) {
	a := &Arbiter{}
	group, track := &producer{}, &producer{}
	a.Took(group)

	a.Stand(true)
	if !group.held() {
		t.Fatal("the group ignored the speaker being taken")
	}

	// A track starting mid-turn must not become audible just because it is newest.
	a.Took(track)
	if !track.held() {
		t.Error("a track that started while the speaker was held began playing")
	}

	a.Stand(false)
	if track.held() {
		t.Error("the track never started: still standing down after the hold ended")
	}
}

// Ducking reaches the ones waiting too, so resuming one mid-turn does not bring it back at full
// volume.
func TestDuckingReachesTheOnesWaiting(t *testing.T) {
	a := &Arbiter{}
	group, track := &producer{}, &producer{}
	a.Took(group)
	a.Took(track)

	a.Duck(true)
	if !group.quiet() || !track.quiet() {
		t.Error("ducking a turn missed a producer")
	}

	a.Duck(false)
	if group.quiet() || track.quiet() {
		t.Error("the turn ended and something stayed quiet")
	}
}

// Ducking only reaches audio written after it. A stream is sent ahead of the clock, so seconds of it
// can already be queued at full volume — which is what "the duck happened, but late" sounds like.
// The one being heard has to go back and re-scale it, and only that one: the others queued nothing,
// and asking them all would attenuate the same samples twice.
func TestOnlyTheProducerBeingHeardRescalesWhatIsQueued(t *testing.T) {
	a := &Arbiter{}
	group, track := &producer{}, &producer{}
	a.Took(group)
	a.Took(track)

	a.Duck(true)

	if got := track.requeued(); got != 1 {
		t.Errorf("the audible producer requeued %d times, want 1", got)
	}
	if got := group.requeued(); got != 0 {
		t.Errorf("a waiting producer requeued %d times, want 0", got)
	}
}

// Coming out of a turn there is nothing to fix: what is queued went in quiet and draining it is the
// end of the duck. Re-scaling by one would be a no-op, and by anything else would be wrong.
func TestNothingIsRescaledWhenTheTurnEnds(t *testing.T) {
	a := &Arbiter{}
	track := &producer{}
	a.Took(track)

	a.Duck(true)
	before := track.requeued()
	a.Duck(false)

	if got := track.requeued(); got != before {
		t.Errorf("unducking requeued: %d then %d", before, got)
	}
}

// A producer starting during a turn has to arrive already quiet, or it blares over the reply.
func TestAProducerStartingMidTurnArrivesQuiet(t *testing.T) {
	a := &Arbiter{}
	a.Duck(true)

	track := &producer{}
	a.Took(track)

	if !track.quiet() {
		t.Error("a track that started mid-turn came in at full volume")
	}
}

// The same producer starting twice — a stream reconnecting, say — must not leave a stale copy behind
// that later gets resumed as if it were still there.
func TestStartingTwiceLeavesOneEntry(t *testing.T) {
	a := &Arbiter{}
	group := &producer{}

	a.Took(group)
	a.Took(group)
	a.Gave(group)

	if a.Playing() != nil {
		t.Error("a duplicate entry outlived the producer that finished")
	}
}

// The media player is one producer that replaces its own track, so every noise or media switch is
// the same stream taking the speaker again. When that happens while the speaker is held, the hold
// already stood the stream down once — standing it down again leaves a resume outstanding, and it
// waits on a gate nobody opens: playing on paper, silent in the room, until the process restarts.
func TestAResumingTrackRetakingTheSpeakerIsNotHeldTwice(t *testing.T) {
	a := &Arbiter{}
	track := &producer{}
	a.Took(track)

	a.Stand(true)
	if !track.held() {
		t.Fatal("the track ignored the speaker being taken")
	}

	// A track change mid-hold: the stream re-joins as itself.
	a.Took(track)
	if !track.held() {
		t.Error("a track must stay stood down while the speaker is held")
	}

	a.Stand(false)
	if track.held() {
		t.Error("the track never came back: still standing down after the hold ended")
	}
}

// A sound that ends during the hold and starts again before it is over is still the producer the
// hold stood down: stopping the noise to start another one must not leave it waiting on a second
// suspend that one resume cannot answer.
func TestARetakeAfterGivingBackDuringAHoldIsNotHeldTwice(t *testing.T) {
	a := &Arbiter{}
	track := &producer{}
	a.Took(track)

	a.Stand(true)
	if !track.held() {
		t.Fatal("the track ignored the speaker being taken")
	}

	// A stop mid-hold, then a new sound before the hold ends.
	a.Gave(track)
	a.Took(track)
	if !track.held() {
		t.Error("a track must stay stood down while the speaker is held")
	}

	a.Stand(false)
	if track.held() {
		t.Error("the track never came back: still standing down after the hold ended")
	}
}

// A producer that takes the speaker back while another is standing down for it is the one heard.
// Leaving both down is the wedge where something reports that it is playing to a silent room.
func TestARetakeIsHeardRatherThanLeftStandingDown(t *testing.T) {
	a := &Arbiter{}
	stream, spin := &producer{}, &producer{}

	a.Took(stream)
	a.Took(spin)
	a.Took(stream)

	if stream.held() {
		t.Error(
			"the producer that took the speaker back is still standing down, so nothing is audible",
		)
	}
	if !spin.held() {
		t.Error("the producer it took over from is still playing")
	}
	if a.Playing() != stream {
		t.Error("the one being heard is not the one that took the speaker last")
	}
}

// Owns is what says whether the audio waiting in the queue may be thrown away. Getting it from a
// producer's own standing is what turns a stop during a reply into a reply cut off mid-word.
func TestTheQueueBelongsToWhateverIsBeingHeard(t *testing.T) {
	a := &Arbiter{}
	track, spin := &producer{}, &producer{}

	a.Took(track)
	if !a.Owns(track) {
		t.Error("the only producer does not own the queue it is filling")
	}

	a.Stand(true)
	if a.Owns(track) {
		t.Error("the queue is not the producer's while the speaker is held")
	}
	a.Stand(false)

	a.Took(spin)
	if a.Owns(track) {
		t.Error("the queue belongs to what took over, not to what stood down for it")
	}
	if !a.Owns(spin) {
		t.Error("the producer being heard does not own the queue")
	}

	a.Gave(spin)
	a.Gave(track)
	if !a.Owns(track) {
		t.Error("with nothing being heard there is no one else's audio to protect")
	}
}
