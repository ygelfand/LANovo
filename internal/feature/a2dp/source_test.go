package a2dp

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/libcountertop/pkg/bluetooth/avrcp"
)

// The phone as the player sees it.
//
// What this has to get right is the mapping, because the two sides name the same things
// differently: AVRCP has one status byte where the player has two booleans, and a status that is
// neither playing nor paused has to come out as neither rather than as stopped-means-paused.

func TestWhatThePlayerIsShown(t *testing.T) {
	state := avrcp.State{
		Track: avrcp.Track{
			Title:  "Aja",
			Artist: "Steely Dan",
			Album:  "Aja",
			Genre:  "Jazz rock",
		},
		Progress: avrcp.Progress{Status: avrcp.StatusPlaying},
	}

	got := nowOf(state, true, nil)

	if !got.Playing || got.Paused {
		t.Errorf("playing=%v paused=%v, want playing", got.Playing, got.Paused)
	}
	if got.Title != "Aja" || got.Artist != "Steely Dan" || got.Album != "Aja" {
		t.Errorf("the card would read %q by %q on %q", got.Title, got.Artist, got.Album)
	}
	// No stop: a phone answers it the way it answers pause.
	if !got.Can.Has(media.CanPause | media.CanNext | media.CanPrevious) {
		t.Errorf("the transport offered is %b, want pause, next and previous", got.Can)
	}
	if got.Can.Has(media.CanStop) {
		t.Error("the card offers stop as well as pause")
	}
}

// Paused is its own state, not playing and not stopped: the card shows a play button and keeps the
// track, where stopped clears it.
func TestPausedIsNeitherPlayingNorStopped(t *testing.T) {
	got := nowOf(avrcp.State{Progress: avrcp.Progress{Status: avrcp.StatusPaused}}, true, nil)

	if got.Playing {
		t.Error("a paused phone reads as playing")
	}
	if !got.Paused {
		t.Error("a paused phone does not read as paused")
	}
}

// With no audio arriving, nothing is playing whatever the phone last said about itself.
func TestEverythingElseIsNeither(t *testing.T) {
	for _, status := range []byte{
		avrcp.StatusStopped, avrcp.StatusForward, avrcp.StatusReverse, avrcp.StatusError,
	} {
		got := nowOf(avrcp.State{Progress: avrcp.Progress{Status: status}}, false, nil)

		if got.Playing || got.Paused {
			t.Errorf("status %#02x with no stream reads playing=%v paused=%v, want neither",
				status, got.Playing, got.Paused)
		}
	}
}

// A phone that said nothing yet is a blank card rather than a crash, and the buttons have to be
// safe to press before anything is connected: the player offers them from whatever it last showed.
func TestNothingConnectedIsBlankAndSafe(t *testing.T) {
	s := source{sink: &Sink{}}

	if got := s.Now(); got.Playing || got.Paused || got.Title != "" {
		t.Errorf("a sink with no phone reads %+v", got)
	}

	s.Play()
	s.Pause()
	s.Stop()
	s.Next()
	s.Previous()
}

// The buttons have to reach the phone rather than do nothing quietly, so each one sends its own
// opcode. A source that mapped two buttons to one opcode would look right on screen and skip the
// wrong way.
func TestEachButtonIsItsOwnOpcode(t *testing.T) {
	seen := map[byte]bool{}
	for _, op := range []byte{
		avrcp.OpPlay, avrcp.OpPause, avrcp.OpStop, avrcp.OpNext, avrcp.OpPrevious,
	} {
		if seen[op] {
			t.Errorf("opcode %#02x is used for two buttons", op)
		}
		seen[op] = true
	}
}

// The stream decides whether this is playing, not the phone's opinion of itself.
//
// A phone that never answered a remote control question leaves its status at stopped. Believing it
// would have the card say nothing is playing while the room listens to music, and the player would
// never come up.
func TestAudioArrivingIsWhatPlayingMeans(t *testing.T) {
	silent := avrcp.State{Progress: avrcp.Progress{Status: avrcp.StatusStopped}}

	if got := nowOf(silent, true, nil); !got.Playing {
		t.Error("audio is arriving and the card says nothing is playing")
	}
	if got := nowOf(silent, false, nil); got.Playing {
		t.Error("no audio is arriving and the card says something is playing")
	}

	// Paused is the one thing the phone knows better: the stream stays up across a pause.
	paused := avrcp.State{Progress: avrcp.Progress{Status: avrcp.StatusPaused}}
	if got := nowOf(paused, true, nil); got.Playing || !got.Paused {
		t.Errorf("a paused phone with the stream up reads playing=%v paused=%v", got.Playing, got.Paused)
	}
}
