package avdtp

import (
	"bytes"
	"errors"
	"testing"
)

func TestAWholePacketRoundTrips(t *testing.T) {
	want := Media{
		Sequence:  0x1234,
		Timestamp: 0xdeadbeef,
		SSRC:      0x01020304,
		Count:     5,
		Payload:   []byte{1, 2, 3, 4},
	}

	got, err := ParseMedia(want.Marshal())
	if err != nil {
		t.Fatalf("ParseMedia: %v", err)
	}

	if got.Sequence != want.Sequence || got.Timestamp != want.Timestamp || got.SSRC != want.SSRC {
		t.Errorf("rtp header came back %+v", got)
	}
	if got.Count != want.Count {
		t.Errorf("%d frames, want %d", got.Count, want.Count)
	}
	if got.Fragmented || got.Start || got.Last {
		t.Errorf("a whole packet came back fragmented: %+v", got)
	}
	if !bytes.Equal(got.Payload, want.Payload) {
		t.Errorf("payload %v, want %v", got.Payload, want.Payload)
	}
}

func TestTheFragmentFlagsRoundTrip(t *testing.T) {
	for _, tc := range []struct{ frag, start, last bool }{
		{true, true, false},
		{true, false, false},
		{true, false, true},
		{true, true, true},
	} {
		m := Media{Fragmented: tc.frag, Start: tc.start, Last: tc.last, Count: 3}

		got, err := ParseMedia(m.Marshal())
		if err != nil {
			t.Fatalf("ParseMedia: %v", err)
		}
		if got.Fragmented != tc.frag || got.Start != tc.start || got.Last != tc.last {
			t.Errorf("%+v came back as fragmented=%v start=%v last=%v",
				tc, got.Fragmented, got.Start, got.Last)
		}
	}
}

// Contributing sources sit between the fixed header and the payload. Not skipping them reads the
// last source as the A2DP header and the rest of it as audio.
func TestContributingSourcesAreSkipped(t *testing.T) {
	packet := make([]byte, rtpHeader+8+1+3)
	packet[0] = rtpVersion<<6 | 2 // two contributing sources
	packet[rtpHeader+8] = 4       // the A2DP header, four frames
	copy(packet[rtpHeader+8+1:], []byte{7, 8, 9})

	m, err := ParseMedia(packet)
	if err != nil {
		t.Fatalf("ParseMedia: %v", err)
	}
	if m.Count != 4 {
		t.Errorf("%d frames, want 4 — the contributing sources were read as the header", m.Count)
	}
	if !bytes.Equal(m.Payload, []byte{7, 8, 9}) {
		t.Errorf("payload %v", m.Payload)
	}
}

func TestAnExtensionHeaderIsSkipped(t *testing.T) {
	// One extension word: four bytes of extension header, then one word of it.
	packet := make([]byte, rtpHeader+4+4+1+2)
	packet[0] = rtpVersion<<6 | 0x10
	packet[rtpHeader+3] = 1 // the extension is one word long
	packet[rtpHeader+8] = 2
	copy(packet[rtpHeader+9:], []byte{5, 6})

	m, err := ParseMedia(packet)
	if err != nil {
		t.Fatalf("ParseMedia: %v", err)
	}
	if m.Count != 2 {
		t.Errorf("%d frames, want 2", m.Count)
	}
	if !bytes.Equal(m.Payload, []byte{5, 6}) {
		t.Errorf("payload %v", m.Payload)
	}
}

func TestAPacketShorterThanItsHeader(t *testing.T) {
	if _, err := ParseMedia(make([]byte, rtpHeader)); !errors.Is(err, ErrShort) {
		t.Errorf("a packet with no a2dp header gave %v, want a short read", err)
	}
}

func TestAnUnknownRTPVersionIsRefused(t *testing.T) {
	packet := make([]byte, rtpHeader+1)
	packet[0] = 1 << 6

	if _, err := ParseMedia(packet); err == nil {
		t.Error("version 1 was accepted")
	}
}

func TestAWholePacketPassesStraightThrough(t *testing.T) {
	var r Reassembler

	got := r.Push(Media{Count: 2, Payload: []byte{1, 2, 3}})
	if !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Errorf("came back %v", got)
	}
}

func TestFragmentsAreJoinedInOrder(t *testing.T) {
	var r Reassembler

	if got := r.Push(Media{Fragmented: true, Start: true, Count: 3, Payload: []byte{1, 2}}); got != nil {
		t.Errorf("the start fragment produced %v", got)
	}
	if got := r.Push(Media{Fragmented: true, Count: 2, Payload: []byte{3, 4}}); got != nil {
		t.Errorf("a middle fragment produced %v", got)
	}

	got := r.Push(Media{Fragmented: true, Last: true, Count: 1, Payload: []byte{5, 6}})
	if !bytes.Equal(got, []byte{1, 2, 3, 4, 5, 6}) {
		t.Errorf("the frame came back %v", got)
	}
}

// A run that ends early has a hole in it. Handing it over decodes to a burst of noise, which
// through a speaker is worse than a frame that never arrives.
func TestARunThatEndsEarlyIsDropped(t *testing.T) {
	var r Reassembler

	r.Push(Media{Fragmented: true, Start: true, Count: 4, Payload: []byte{1, 2}})
	r.Push(Media{Fragmented: true, Count: 3, Payload: []byte{3, 4}})

	if got := r.Push(Media{Fragmented: true, Last: true, Count: 2, Payload: []byte{5}}); got != nil {
		t.Errorf("a run one fragment short came back as %v", got)
	}
}

// A fragment arriving with no start is a run this side joined halfway through, so there is no way
// to tell which frame the bytes belong to.
func TestAFragmentWithNoStartIsDropped(t *testing.T) {
	var r Reassembler

	if got := r.Push(Media{Fragmented: true, Count: 2, Payload: []byte{1, 2}}); got != nil {
		t.Errorf("came back %v", got)
	}
	if got := r.Push(Media{Fragmented: true, Last: true, Count: 1, Payload: []byte{3}}); got != nil {
		t.Errorf("came back %v", got)
	}
}

// A new start abandons whatever was in hand rather than joining onto it.
func TestAnAbandonedRunDoesNotLeakIntoTheNext(t *testing.T) {
	var r Reassembler

	r.Push(Media{Fragmented: true, Start: true, Count: 3, Payload: []byte{0xaa, 0xbb}})
	r.Push(Media{Fragmented: true, Start: true, Count: 2, Payload: []byte{1, 2}})

	got := r.Push(Media{Fragmented: true, Last: true, Count: 1, Payload: []byte{3, 4}})
	if !bytes.Equal(got, []byte{1, 2, 3, 4}) {
		t.Errorf("the frame came back %v, so the abandoned run leaked in", got)
	}
}

// A whole packet in the middle of a run means the run is never finishing.
func TestAWholePacketClearsAPartialRun(t *testing.T) {
	var r Reassembler

	r.Push(Media{Fragmented: true, Start: true, Count: 3, Payload: []byte{0xaa}})
	r.Push(Media{Count: 1, Payload: []byte{1, 2}})

	if got := r.Push(Media{Fragmented: true, Last: true, Count: 1, Payload: []byte{3}}); got != nil {
		t.Errorf("the abandoned run came back as %v", got)
	}
}

func TestAFrameFragmentedIntoOnePiece(t *testing.T) {
	var r Reassembler

	got := r.Push(Media{Fragmented: true, Start: true, Last: true, Count: 1, Payload: []byte{1, 2}})
	if !bytes.Equal(got, []byte{1, 2}) {
		t.Errorf("came back %v", got)
	}
}
