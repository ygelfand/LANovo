package a2dp

import (
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// square stands in for a cover that arrived.
func square() *ui.Image { return ui.NewImage(4, 4, theme.Color{}) }

// One image at a time. An image arrives over as many answers as it takes and none of them says
// which image it is, so a second fetch started over the first assembles the two into each other.

// connected puts the conversation where a fetch can start, with nothing on the link to send over.
func connected(t *testing.T) *Sink {
	t.Helper()
	t.Cleanup(art.clear)

	art.clear()
	art.mu.Lock()
	art.connection, art.open = 1, true
	art.mu.Unlock()

	return &Sink{}
}

// waiting is the handles still to be fetched, and busy the one in flight.
func waiting() (busy string, want []string) {
	art.mu.Lock()
	defer art.mu.Unlock()
	return art.asked, append([]string(nil), art.want...)
}

func TestOneFetchAtATime(t *testing.T) {
	s := connected(t)

	s.wants("111", false)
	s.wants("222", false)

	busy, want := waiting()
	if busy != "111" {
		t.Fatalf("fetching %q, want the first", busy)
	}
	if len(want) != 2 {
		t.Fatalf("%d waiting, want both until one finishes", len(want))
	}

	s.fetched("111", square())

	busy, want = waiting()
	if busy != "222" {
		t.Errorf("fetching %q after the first finished, want the second", busy)
	}
	if len(want) != 1 || want[0] != "222" {
		t.Errorf("waiting %v, want only the second", want)
	}
}

// What is playing is fetched before what is merely next, once whatever is in flight has finished.
func TestTheOneBeingPlayedGoesNext(t *testing.T) {
	s := connected(t)

	s.wants("111", false)
	s.wants("222", false)
	s.wants("333", true)

	if busy, _ := waiting(); busy != "111" {
		t.Fatalf("fetching %q, want the one already in flight", busy)
	}

	s.fetched("111", square())

	if busy, _ := waiting(); busy != "333" {
		t.Errorf("fetching %q next, want the one being played", busy)
	}
}

// A handle already held is not fetched again, and neither is one already waiting.
func TestNothingIsFetchedTwice(t *testing.T) {
	s := connected(t)

	s.wants("111", false)
	s.wants("111", false)
	s.wants("222", false)

	if _, want := waiting(); len(want) != 2 {
		t.Fatalf("waiting %v, want one of each", want)
	}

	s.fetched("111", square())
	s.fetched("222", square())

	s.wants("111", false)
	if busy, want := waiting(); busy != "" || len(want) != 0 {
		t.Errorf("a handle already held was asked for again: %q %v", busy, want)
	}
}

// A fetch that produced nothing still ends, or the queue stops there.
func TestARefusedImageDoesNotWedgeTheQueue(t *testing.T) {
	s := connected(t)

	s.wants("111", false)
	s.wants("222", false)
	s.fetched("111", nil)

	if busy, _ := waiting(); busy != "222" {
		t.Errorf("fetching %q after a refusal, want the next", busy)
	}
	if s.image("111") != nil {
		t.Error("a refused handle has an image")
	}
}

// A fetch that is never answered must not hold the queue for the life of the link.
func TestAFetchThatIsNeverAnsweredLetsGo(t *testing.T) {
	s := connected(t)

	s.wants("111", false)
	s.wants("222", false)

	if busy, _ := waiting(); busy != "111" {
		t.Fatalf("fetching %q, want the first", busy)
	}

	art.mu.Lock()
	art.began = time.Now().Add(-waits - time.Second)
	art.mu.Unlock()

	s.pump()

	if busy, _ := waiting(); busy != "222" {
		t.Errorf("fetching %q, want the next after the first went unanswered", busy)
	}
}

func TestWhatIsNotWantedIsDropped(t *testing.T) {
	s := connected(t)

	s.wants("111", false)
	s.fetched("111", square())

	if s.image("111") == nil {
		t.Fatal("a fetched image was not kept")
	}

	art.mu.Lock()
	art.have["111"].used = time.Now().Add(-stays - time.Minute)
	art.mu.Unlock()

	s.expire()

	if s.image("111") != nil {
		t.Error("an image nothing has wanted for a long time is still held")
	}
}

// Wanting one again keeps it, so a queue going round does not fetch the same covers forever.
func TestWantingOneAgainKeepsIt(t *testing.T) {
	s := connected(t)

	s.wants("111", false)
	s.fetched("111", square())

	art.mu.Lock()
	art.have["111"].used = time.Now().Add(-stays - time.Minute)
	art.mu.Unlock()

	s.wants("111", false)
	s.expire()

	if s.image("111") == nil {
		t.Error("an image wanted again was dropped anyway")
	}
}
