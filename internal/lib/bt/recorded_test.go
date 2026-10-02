package bt

import (
	"bytes"
	"testing"

	"github.com/ygelfand/LANovo/internal/lib/bt/l2cap"
)

// A real phone's bytes, replayed.
//
// The scripted phone in sink_test is written from the same reading of the spec as the code it
// drives, so the two agree wherever that reading is wrong. Two bugs got through it that way: SDP
// attribute identifiers written one byte wide, and a configuration response looked up by the far
// end's channel number instead of ours. Both were invisible because the fake phone made the same
// mistakes.
//
// These are bytes a handset actually sent, taken from a log on 2026-09-21. Nothing here is
// reconstructed from what the code expects.

// TestARealPhoneOpensTheStream replays the sequence that ends in AVDTP, which is the one that
// failed: SDP answered, a channel to the sink opened, and then Discover on it.
func TestARealPhoneOpensTheStream(t *testing.T) {
	p := newPhone(t)

	// Their channel numbers, as the handset chose them.
	const theirSDP, theirSink = 0x005c, 0x007e

	// SDP first, which is how it finds us.
	sdpCID := opens(t, p, l2cap.PSMSDP, theirSDP)

	// Searching for everything that speaks L2CAP, which is how a phone browses. The handset sent
	// exactly these twenty bytes.
	out := p.wire(l2cap.Frame{CID: sdpCID, Payload: []byte{
		0x06, 0x00, 0x00, 0x00, 0x0f,
		0x35, 0x03, 0x19, 0x01, 0x00,
		0x03, 0xf0,
		0x35, 0x05, 0x0a, 0x00, 0x00, 0xff, 0xff, 0x00,
	}})

	if len(out) != 1 {
		t.Fatalf("a browse got %d answers", len(out))
	}

	// It has to carry the sink's class and a psm two bytes wide, or the phone has nothing to
	// connect to. This is the shape the first fix was about.
	said := out[0].Payload
	for _, want := range []struct {
		what  string
		bytes []byte
	}{
		{"an audio sink", []byte{0x19, 0x11, 0x0b}},
		{"a two byte attribute id", []byte{0x09, 0x00, 0x04}},
		{"L2CAP carrying psm 0x19", []byte{0x19, 0x01, 0x00, 0x09, 0x00, 0x19}},
	} {
		if !bytes.Contains(said, want.bytes) {
			t.Errorf("the browse answer does not carry %s as % x", want.what, want.bytes)
		}
	}

	// Then it connects to the psm it just read, and that channel has to actually open.
	sinkCID := opens(t, p, SinkPSM, theirSink)

	if ch := p.sink.l2.Channel(sinkCID); ch == nil || ch.State != l2cap.Open {
		t.Fatalf("the sink channel is %v after both ends configured, want open", ch.State)
	}

	// Discover, which is the first thing AVDTP says and what came back unroutable.
	out = p.wire(l2cap.Frame{CID: sinkCID, Payload: []byte{0x00, 0x01}})
	if len(out) != 1 {
		t.Fatalf("Discover got %d answers, want one", len(out))
	}

	// A response to the same transaction, carrying an endpoint.
	got := out[0].Payload
	if len(got) < 4 {
		t.Fatalf("Discover was answered with % x", got)
	}
	if got[0]&0x03 != 0x02 {
		t.Errorf("Discover was answered with message type %#02x, want a response accept", got[0]&0x03)
	}
	if got[1] != 0x01 {
		t.Errorf("the answer is to signal %#02x, want Discover", got[1])
	}
}

// opens runs the connect and configure the handset sends, and returns the channel we allocated.
//
// The configuration response names our channel rather than theirs, which is what the handset does
// and what the second fix was about.
func opens(t *testing.T, p *phone, psm, theirs uint16) uint16 {
	t.Helper()

	out := p.commands(l2cap.Connect{PSM: psm, SourceCID: theirs}.Command(p.nextID()))
	if len(out) != 2 {
		t.Fatalf("connecting to %#x got %d commands, want an answer and a configuration", psm, len(out))
	}

	connected, err := l2cap.ParseConnected(out[0])
	if err != nil {
		t.Fatalf("ParseConnected: %v", err)
	}
	if connected.Result != l2cap.ConnectSuccess {
		t.Fatalf("connecting to %#x was refused: %#x", psm, connected.Result)
	}

	ours := connected.DestinationCID

	// Their configuration of us, then their answer to ours.
	p.commands(l2cap.Configure{
		DestinationCID: ours,
		Options:        []l2cap.Option{l2cap.MTUOption(1024)},
	}.Command(p.nextID()))

	p.commands(l2cap.Configured{
		SourceCID: ours,
		Result:    l2cap.ConfigSuccess,
	}.Command(out[1].ID))

	return ours
}
