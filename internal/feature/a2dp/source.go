package a2dp

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/libcountertop/pkg/bluetooth"
	"github.com/ygelfand/libcountertop/pkg/bluetooth/avrcp"
	"github.com/ygelfand/libcountertop/pkg/bluetooth/l2cap"
	"github.com/ygelfand/libcountertop/pkg/bluetooth/sdp"
)

// A connected phone behind media.Source, so the player shows it like any other.
//
// The phone is the A2DP source but the AVRCP target: it holds the media state and this device asks
// for it. The asking is internal/lib/bt/avrcp's; this is the join.

// source is the live phone, behind media.Source.
type source struct{ sink *Sink }

// Now is what the phone last said it was playing.
func (s source) Now() media.Now {
	remote := s.sink.remote()
	if remote == nil {
		return media.Now{}
	}

	state := remote.State()
	now := nowOf(state, s.sink.playing(), s.sink.queued(state))
	now.Art = s.sink.cover(state.Track.Art)
	return now
}

// playing is whether audio is arriving from the phone.
func (s *Sink) playing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streaming
}

// nowOf is what the player shows, from what the phone last said and whether audio is arriving.
//
// The stream decides whether this is playing, not the phone's idea of it. A phone that never
// answered a remote control question leaves the status at stopped, and a card that believed it
// would say nothing is playing while the room listens to music.
func nowOf(state avrcp.State, streaming bool, queue []media.Track) media.Now {
	return media.Now{
		Playing: streaming && state.Progress.Status != avrcp.StatusPaused,
		Paused:  state.Progress.Status == avrcp.StatusPaused,

		Title:  state.Track.Title,
		Artist: state.Track.Artist,
		Album:  state.Track.Album,

		// The phone answers with where it was when it last said so. It sends again on a seek and
		// at whatever interval it registered for, so this steps rather than runs.
		Elapsed: state.Progress.Position,
		Length:  state.Progress.Length,

		// The artwork is fetched over a channel of its own and put on by the caller, which is the
		// only part of this that is not in the answer being read.
		Queue: queue,

		// No stop: a phone answers it the same way it answers pause, and the card would offer the
		// same button twice.
		Can: media.CanPause | media.CanNext | media.CanPrevious,
	}
}

func (source) Kind() media.Kind { return media.FromBluetooth }
func (s source) Label() string  { return s.sink.Label() }

func (s source) Play()     { s.sink.press(avrcp.OpPlay) }
func (s source) Pause()    { s.sink.press(avrcp.OpPause) }
func (s source) Stop()     { s.sink.press(avrcp.OpStop) }
func (s source) Next()     { s.sink.press(avrcp.OpNext) }
func (s source) Previous() { s.sink.press(avrcp.OpPrevious) }

// remote is the live controller, or nil when no phone is connected.
func (s *Sink) remote() *avrcp.Controller {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sink == nil {
		return nil
	}
	return s.sink.Remote()
}

// press sends a transport button to the phone.
func (s *Sink) press(op byte) {
	s.tell(fmt.Sprintf("button %#02x", op), func(sink *bt.Sink) ([]l2cap.Frame, error) {
		return sink.Press(op)
	})
}

// ask asks the phone what it is playing and to say when it changes.
func (s *Sink) ask() {
	s.tell("asking what is playing", (*bt.Sink).Ask)
}

// traced is when frame logging stops, as a wall clock in nanoseconds. Zero is never.
//
// A deadline rather than a switch, so a trace left on goes quiet by itself. Every non-audio frame
// is a line, which is fine for a minute and not for a day.
var traced atomic.Int64

// Trace logs every frame but the audio for a while.
func Trace(within time.Duration) { traced.Store(time.Now().Add(within).UnixNano()) }

// tracing reports whether a frame is worth a line right now.
func tracing() bool {
	until := traced.Load()
	return until != 0 && time.Now().UnixNano() < until
}

// Attributes asks about the current track, naming which attributes are wanted. The answer arrives
// on the channel like any other, so it shows up in a trace rather than coming back from here.
func (s *Sink) Attributes(ids ...uint32) {
	s.tell(fmt.Sprintf("asking for attributes %v", ids), func(sink *bt.Sink) ([]l2cap.Frame, error) {
		return sink.Attributes(ids...)
	})
}

// Browsing asks the phone for the browsing channel by hand, for a link where the one asked for
// when the remote control channel opened was refused.
func (s *Sink) Browsing() { s.open() }

// Queue asks again for what is lined up, which otherwise happens on its own when the track changes.
func (s *Sink) Queue() { s.list() }

// Look asks the phone what it serves under a class, which is how the channel cover art comes over
// is learned rather than assumed. The number is allocated when the far end's service starts, so it
// is not the same twice.
func (s *Sink) Look(class uint16) {
	s.tell(fmt.Sprintf("looking up %#04x", class), func(sink *bt.Sink) ([]l2cap.Frame, error) {
		return sink.Discover(sdp.Search{
			Pattern:    []uint32{uint32(class)},
			MaxBytes:   0x03f0,
			Attributes: []sdp.AttrRange{{First: 0x0000, Last: 0xffff}},
		})
	})
}

// Looking asks for a channel to the phone's service records.
func (s *Sink) Looking() {
	s.tell("opening a lookup channel", (*bt.Sink).OpenDiscovery)
}

// look asks what the phone serves, opening the channel to ask on where there is not one yet. The
// question goes out when the channel answers.
func (s *Sink) look() {
	s.mu.Lock()
	sink := s.sink
	s.mu.Unlock()

	if sink == nil {
		return
	}
	if !sink.Finding() {
		s.Looking()
		return
	}
	// The target's record, because the images belong to whatever is playing them.
	s.Look(sdp.UUIDAVRemoteControlTarget)
}

// tell puts something this device wants to say on the link, rather than answering something said.
func (s *Sink) tell(what string, build func(*bt.Sink) ([]l2cap.Frame, error)) {
	s.mu.Lock()
	sink, send, handle := s.sink, s.send, s.handle
	s.mu.Unlock()

	if sink == nil || send == nil || handle == 0 {
		return
	}

	frames, err := build(sink)
	if err != nil {
		slog.Warn("bluetooth not sent", "what", what, "err", err)
		return
	}

	for _, f := range frames {
		if tracing() {
			slog.Info("bluetooth sent", "what", what,
				"cid", fmt.Sprintf("%#04x", f.CID), "pdu", brief(f.Payload))
		}

		if err := s.put(send, handle, f.Marshal()); err != nil {
			slog.Warn("bluetooth not sent", "what", what, "err", err)
			return
		}
	}
}

// put sends a frame, or holds it behind the answer to the frame being handled.
func (s *Sink) put(send func(uint16, []byte) error, handle uint16, pdu []byte) error {
	s.mu.Lock()
	if s.answering {
		s.held = append(s.held, pdu)
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	return send(handle, pdu)
}

// attach gives the card to the phone.
func (s *Sink) attach() {
	media.Get().Began(source{sink: s})
}

// detach gives the card back, for a phone that has gone.
func (s *Sink) detach() {
	media.Get().Ended(source{sink: s})
	media.Get().Release(source{sink: s})
}
