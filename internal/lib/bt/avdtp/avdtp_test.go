package avdtp

import (
	"bytes"
	"errors"
	"testing"
)

func TestAMessageGoesOutAndComesBack(t *testing.T) {
	want := Message{Label: 5, Type: Command, Signal: SignalGetCapabilities, Data: []byte{0x04}}

	got, err := ParseMessage(want.Marshal())
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	if got.Label != want.Label || got.Type != want.Type || got.Signal != want.Signal {
		t.Errorf("came back %+v, want %+v", got, want)
	}
	if !bytes.Equal(got.Data, want.Data) {
		t.Errorf("data came back %x", got.Data)
	}
}

// The label is four bits and wraps, and an answer carrying the wrong one is given to the wrong
// question.
func TestTheLabelSurvivesEveryValue(t *testing.T) {
	for label := byte(0); label < 16; label++ {
		got, err := ParseMessage(Message{Label: label, Signal: SignalStart}.Marshal())
		if err != nil {
			t.Fatalf("label %d: %v", label, err)
		}
		if got.Label != label {
			t.Errorf("label %d came back %d", label, got.Label)
		}
	}
}

// The signal is six bits sharing a byte with two reserved ones, so a parser that takes the whole
// byte reads a different signal whenever those are set.
func TestTheSignalIsSixBits(t *testing.T) {
	raw := Message{Label: 1, Signal: SignalStart}.Marshal()
	raw[1] |= 0xc0 // the two reserved bits, which a sender is allowed to set

	got, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	if got.Signal != SignalStart {
		t.Errorf("the signal came back %#x with the reserved bits set", got.Signal)
	}
}

// A fragmented message is refused rather than read as a short one. Silently truncating a capability
// list configures the stream wrong, which is worse than not answering.
func TestAFragmentedMessageIsRefused(t *testing.T) {
	raw := Message{Label: 1, Signal: SignalGetAllCaps}.Marshal()
	raw[0] = raw[0]&^(0x3<<2) | PacketStart<<2

	if _, err := ParseMessage(raw); err == nil {
		t.Error("a fragmented message was read as a whole one")
	}
}

func TestAMessageThatHasNotAllArrived(t *testing.T) {
	if _, err := ParseMessage(nil); !errors.Is(err, ErrShort) {
		t.Errorf("nothing gave %v, want a short read", err)
	}
	if _, err := ParseMessage([]byte{0x10}); !errors.Is(err, ErrShort) {
		t.Errorf("one byte gave %v, want a short read", err)
	}
}

// An answer carries the question's label and signal, or the phone cannot tell what was answered.
func TestAnAnswerCarriesTheQuestion(t *testing.T) {
	q := Message{Label: 9, Type: Command, Signal: SignalSetConfig}

	ok := q.Accept()
	if ok.Label != 9 || ok.Signal != SignalSetConfig || ok.Type != ResponseAccept {
		t.Errorf("the acceptance came out %+v", ok)
	}

	no := q.Reject(ErrorBadState)
	if no.Type != ResponseReject || no.Data[0] != ErrorBadState {
		t.Errorf("the refusal came out %+v", no)
	}

	// The configuration signals name the category that was wrong first, which is what lets a phone
	// try again rather than give up.
	bad := q.RejectConfig(CatMediaCodec, ErrorUnsupportedConfig)
	if len(bad.Data) != 2 || bad.Data[0] != CatMediaCodec || bad.Data[1] != ErrorUnsupportedConfig {
		t.Errorf("the configuration refusal came out %x", bad.Data)
	}
}

// One endpoint, advertised so a phone can address it.
func TestEndpointsGoOutAndComeBack(t *testing.T) {
	want := SEP{SEID: 1, Media: MediaAudio, TSEP: Sink}

	got, err := ParseSEPs(want.Marshal())
	if err != nil {
		t.Fatalf("ParseSEPs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("%d endpoints came back, want one", len(got))
	}
	if got[0] != want {
		t.Errorf("came back %+v, want %+v", got[0], want)
	}
}

func TestAnEndpointInUseSaysSo(t *testing.T) {
	got, err := ParseSEPs(SEP{SEID: 3, InUse: true, Media: MediaAudio, TSEP: Sink}.Marshal())
	if err != nil {
		t.Fatalf("ParseSEPs: %v", err)
	}
	if !got[0].InUse {
		t.Error("an endpoint in use did not say so")
	}
	if got[0].SEID != 3 {
		t.Errorf("the endpoint id came back %d", got[0].SEID)
	}
}

func TestAnEndpointListThatDoesNotAddUp(t *testing.T) {
	if _, err := ParseSEPs([]byte{0x04}); err == nil {
		t.Error("half an endpoint was accepted")
	}
}

// Most commands name the endpoint they are for in the top six bits of their first byte.
func TestReadingTheEndpointACommandIsFor(t *testing.T) {
	got, ok := SEID([]byte{AddressTo(7)})
	if !ok {
		t.Fatal("no endpoint in the command")
	}
	if got != 7 {
		t.Errorf("the command addressed endpoint %d, want 7", got)
	}
	if _, ok := SEID(nil); ok {
		t.Error("an empty command named an endpoint")
	}
}
