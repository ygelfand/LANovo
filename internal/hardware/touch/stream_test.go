package touch

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/board"
)

// The decoder and the recognizer are each tested on their own, with contacts made up to suit. They
// meet nowhere else, so a swipe here is driven from kernel events all the way to a gesture.
func TestASwipeFromEventsToAGesture(t *testing.T) {
	w, h := board.Current().PanelHeight, board.Current().PanelWidth

	var d decoder
	r := NewRecognizer(w, h)

	var got Gesture
	var fired bool

	drag := func(events ...rawEvent) {
		for _, e := range events {
			for _, c := range d.event(e) {
				if g, ok := r.Feed(c); ok {
					got, fired = g, true
				}
			}
		}
	}

	// In from the left of the picture. The panel reports its native portrait, where that is a high
	// Y counting down.
	drag(
		ev(evAbs, absMTTrackingID, 4),
		ev(evAbs, absMTPositionX, 600),
		ev(evAbs, absMTPositionY, 1900),
		syn(),
	)
	drag(ev(evAbs, absMTPositionY, 1500), syn())
	drag(ev(evAbs, absMTPositionY, 1100), syn())
	drag(ev(evAbs, absMTTrackingID, released), syn())

	if !fired {
		t.Fatal("a swipe across the glass produced no gesture")
	}
	if got.Kind != Swipe {
		t.Errorf("kind = %v, want a swipe", got.Kind)
	}
	if got.From != Left {
		t.Errorf("from = %v, want the left edge", got.From)
	}
	if got.Toward != Right {
		t.Errorf("toward = %v, want the right", got.Toward)
	}
}

func TestATapFromEventsToAGesture(t *testing.T) {
	w, h := board.Current().PanelHeight, board.Current().PanelWidth

	var d decoder
	r := NewRecognizer(w, h)

	var got Gesture
	var fired bool

	for _, e := range []rawEvent{
		ev(evAbs, absMTTrackingID, 9),
		ev(evAbs, absMTPositionX, 600),
		ev(evAbs, absMTPositionY, 960),
		syn(),
		ev(evAbs, absMTTrackingID, released),
		syn(),
	} {
		for _, c := range d.event(e) {
			if g, ok := r.Feed(c); ok {
				got, fired = g, true
			}
		}
	}

	if !fired {
		t.Fatal("a tap on the glass produced no gesture")
	}
	if got.Kind != Tap {
		t.Errorf("kind = %v, want a tap", got.Kind)
	}
}

// Two fingers in two slots, lifted in the order they went down. Each has to come back as its own
// gesture rather than one matching the other's journey.
func TestTwoFingersEachGetTheirOwnGesture(t *testing.T) {
	var d decoder
	r := NewRecognizer(board.Current().PanelHeight, board.Current().PanelWidth)

	var got []Gesture
	drag := func(events ...rawEvent) {
		for _, e := range events {
			for _, c := range d.event(e) {
				if g, ok := r.Feed(c); ok {
					got = append(got, g)
				}
			}
		}
	}

	drag(
		ev(evAbs, absMTSlot, 0), ev(evAbs, absMTTrackingID, 1),
		ev(evAbs, absMTPositionX, 300), ev(evAbs, absMTPositionY, 400),
		ev(evAbs, absMTSlot, 1), ev(evAbs, absMTTrackingID, 2),
		ev(evAbs, absMTPositionX, 900), ev(evAbs, absMTPositionY, 1400),
		syn(),
	)
	drag(ev(evAbs, absMTSlot, 0), ev(evAbs, absMTTrackingID, released), syn())
	drag(ev(evAbs, absMTSlot, 1), ev(evAbs, absMTTrackingID, released), syn())

	if len(got) != 2 {
		t.Fatalf("got %d gestures, want one per finger", len(got))
	}
}
