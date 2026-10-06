package a2dp

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/bluetooth/pair"
	"github.com/ygelfand/libcountertop/pkg/bluetooth/sbc"
)

// The wiring, checked where it can be without a radio.
//
// What this package does is join parts that are each tested on their own, so what is worth testing
// here is the joins: the shape audio comes out as, and that the three things a link asks of it
// answer rather than panic before a stream has been agreed.

// A phone sending mono has to come out of both speakers. Handing the card one channel's worth where
// it expects two plays it at double speed out of one side.
func TestMonoIsSentToBothEars(t *testing.T) {
	got := interleave(sbc.Samples{{1, 2, 3}})

	want := []int16{1, 1, 2, 2, 3, 3}
	if len(got) != len(want) {
		t.Fatalf("%d samples from 3 mono ones, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d is %d, want %d", i, got[i], want[i])
		}
	}
}

// Stereo goes out left, right, left, right, which is what the card reads.
func TestStereoIsInterleaved(t *testing.T) {
	got := interleave(sbc.Samples{{1, 3, 5}, {2, 4, 6}})

	want := []int16{1, 2, 3, 4, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("%d samples, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d is %d, want %d", i, got[i], want[i])
		}
	}
}

func TestNoAudioIsNoSamples(t *testing.T) {
	if got := interleave(nil); got != nil {
		t.Errorf("came back with %d samples", len(got))
	}
	if got := interleave(sbc.Samples{}); got != nil {
		t.Errorf("came back with %d samples", len(got))
	}
}

// Everything a link asks for arrives before a stream is agreed, and some of it arrives after one
// has ended. None of it may panic on a sink that has not been built.
func TestAskingBeforeAnythingIsAgreed(t *testing.T) {
	var s Sink

	said, err := s.Event(pair.Event{Code: pair.EventConnectionRequest})
	if err != nil {
		t.Errorf("an event before the stack was built: %v", err)
	}
	if len(said) != 0 {
		t.Errorf("%d commands from a sink with no pairing manager", len(said))
	}

	out, err := s.Data(1, []byte{0x04, 0x00, 0x40, 0x00, 1, 2, 3, 4})
	if err != nil {
		t.Errorf("data before the stack was built: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("%d replies from a sink that is not up", len(out))
	}

	// And a link ending before one ever started.
	s.Gone(1)
}

// The reader is shared, so a disconnection on any link arrives here. Only the one this sink is on
// tears it down: the others belong to whatever else the radio is carrying.
func TestAnotherLinkEndingIsNotOurs(t *testing.T) {
	s := &Sink{}
	s.start("test")

	// Whatever it says: the handle is taken from data arriving, and this is data arriving.
	s.Data(7, []byte{0x04, 0x00, 0x40, 0x00, 1, 2, 3, 4})

	s.Gone(9)
	if s.handle != 7 {
		t.Errorf("a stranger's link ended and this one is now %d", s.handle)
	}

	s.Gone(7)
	if s.handle != 0 {
		t.Errorf("this link ended and the handle is still %d", s.handle)
	}
}

// A malformed PDU is reported rather than answered. The link stays up: one bad packet is a glitch,
// and dropping the connection over it is how a stutter becomes a phone that stops playing.
func TestAMalformedPDUIsRefusedNotAnswered(t *testing.T) {
	s := &Sink{}
	s.start("test")

	for _, pdu := range [][]byte{
		{},
		{0x01},
		{0xff, 0xff, 0x40, 0x00},
	} {
		out, err := s.Data(1, pdu)
		if err == nil && len(out) > 0 {
			t.Errorf("% x was answered with %d frames", pdu, len(out))
		}
	}
}

// The phone's volume slider has to reach the speaker.
//
// Everything underneath was already there: the remote control accepts SetAbsoluteVolume, answers it
// and keeps the value, which is why this looked finished. What was missing is the one assignment
// that connects it to anything, so moving the slider changed the number on the phone and nothing
// here. internal/lib/bt/avrcp covers the protocol; this covers the join.
func TestThePhonesVolumeIsWiredToTheSpeaker(t *testing.T) {
	s := &Sink{}
	s.start("volume test")

	if s.sink.Remote().Volume == nil {
		t.Fatal("nothing is listening for the phone's volume")
	}
}

// Roles are split on this channel and both halves have to be reachable: this device is the
// controller for transport and metadata, and the target for volume. A sink that only built one of
// them answers half the profile.
func TestBothRemoteControlRolesAreBuilt(t *testing.T) {
	s := &Sink{}
	s.start("roles test")

	if s.sink.Remote() == nil {
		t.Fatal("there is no remote control")
	}
	if got := s.sink.Remote().State().Volume; got != -1 {
		t.Errorf("a fresh remote reports volume %d, want -1 for never said", got)
	}
}
