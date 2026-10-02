package l2cap

import (
	"bytes"
	"errors"
	"testing"
)

func TestAFrameGoesOutAndComesBackTheSame(t *testing.T) {
	want := Frame{CID: 0x0040, Payload: []byte{1, 2, 3, 4, 5}}

	got, err := ParseFrame(want.Marshal())
	if err != nil {
		t.Fatalf("ParseFrame: %v", err)
	}
	if got.CID != want.CID || !bytes.Equal(got.Payload, want.Payload) {
		t.Errorf("came back %+v, want %+v", got, want)
	}
}

// A frame arrives whole — whatever reassembled it already waited. So a length that disagrees with
// what is there is a malformed frame, not more to read.
func TestAFrameWhoseLengthDisagreesWithItself(t *testing.T) {
	whole := Frame{CID: 0x0040, Payload: []byte{1, 2, 3, 4}}.Marshal()

	if _, err := ParseFrame(whole[:len(whole)-1]); err == nil {
		t.Error("a frame one byte short of its length was accepted")
	}
	if _, err := ParseFrame(append(whole, 0xff)); err == nil {
		t.Error("a frame one byte past its length was accepted")
	}
	if _, err := ParseFrame(whole[:2]); !errors.Is(err, ErrShort) {
		t.Errorf("half a header gave %v, want a short read", err)
	}
}

func TestAnEmptyFrameIsStillAFrame(t *testing.T) {
	got, err := ParseFrame(Frame{CID: CIDSignalling}.Marshal())
	if err != nil {
		t.Fatalf("ParseFrame: %v", err)
	}
	if got.CID != CIDSignalling || len(got.Payload) != 0 {
		t.Errorf("came back %+v", got)
	}
}

// Several commands may share one signalling frame, and the identifier is what pairs each answer
// with its question.
func TestSeveralCommandsInOneFrame(t *testing.T) {
	want := []Command{
		{Code: CodeConnectRequest, ID: 1, Data: []byte{0x19, 0x00, 0x41, 0x00}},
		{Code: CodeEchoRequest, ID: 2, Data: nil},
		{Code: CodeInfoRequest, ID: 3, Data: []byte{0x02, 0x00}},
	}

	frame := Signal(want...)
	if frame.CID != CIDSignalling {
		t.Errorf("signalling went out on channel %#x", frame.CID)
	}

	got, err := ParseCommands(frame.Payload)
	if err != nil {
		t.Fatalf("ParseCommands: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("%d commands came back, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i].Code != want[i].Code || got[i].ID != want[i].ID {
			t.Errorf("command %d came back %+v, want %+v", i, got[i], want[i])
		}
		if !bytes.Equal(got[i].Data, want[i].Data) {
			t.Errorf("command %d data came back %x, want %x", i, got[i].Data, want[i].Data)
		}
	}
}

func TestSignallingThatDoesNotAddUp(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{"nothing at all", nil},
		{"half a header", []byte{CodeEchoRequest, 1}},
		{"says more than it carries", []byte{CodeConnectRequest, 1, 0x04, 0x00, 0x19, 0x00}},
		{"trailing scrap", []byte{CodeEchoRequest, 1, 0x00, 0x00, 0xff}},
	} {
		if _, err := ParseCommands(tc.payload); err == nil {
			t.Errorf("%s: was accepted", tc.name)
		}
	}
}

func TestReadingAConnectionRequest(t *testing.T) {
	c := Command{Code: CodeConnectRequest, ID: 7, Data: []byte{0x19, 0x00, 0x41, 0x00}}

	got, err := ParseConnect(c)
	if err != nil {
		t.Fatalf("ParseConnect: %v", err)
	}
	if got.PSM != PSMAVDTP {
		t.Errorf("psm came back %#x, want avdtp", got.PSM)
	}
	if got.SourceCID != 0x0041 {
		t.Errorf("source channel came back %#x", got.SourceCID)
	}
}

// A channel has a different number at each end, and answering with the wrong one in the wrong slot
// is a channel neither side can address.
func TestAConnectionResponseCarriesBothChannelNumbers(t *testing.T) {
	want := Connected{DestinationCID: 0x0040, SourceCID: 0x0041, Result: ConnectSuccess}

	got, err := ParseConnected(want.Command(7))
	if err != nil {
		t.Fatalf("ParseConnected: %v", err)
	}
	if got != want {
		t.Errorf("came back %+v, want %+v", got, want)
	}
	if c := want.Command(7); c.Code != CodeConnectResponse || c.ID != 7 {
		t.Errorf("the command came out as %#x id %d", c.Code, c.ID)
	}
}

func TestRefusingAChannelNobodyServes(t *testing.T) {
	refused := Connected{SourceCID: 0x0041, Result: ConnectBadPSM}

	got, err := ParseConnected(refused.Command(9))
	if err != nil {
		t.Fatalf("ParseConnected: %v", err)
	}
	if got.Result != ConnectBadPSM {
		t.Errorf("result came back %#x, want the psm refusal", got.Result)
	}
	if got.DestinationCID != 0 {
		t.Errorf("a refused channel was given number %#x", got.DestinationCID)
	}
}

func TestDisconnectGoesBothWays(t *testing.T) {
	want := Disconnect{DestinationCID: 0x0040, SourceCID: 0x0041}

	for _, code := range []byte{CodeDisconnectRequest, CodeDisconnectResponse} {
		c := want.Command(code, 3)
		if c.Code != code {
			t.Errorf("code came out %#x, want %#x", c.Code, code)
		}

		got, err := ParseDisconnect(c)
		if err != nil {
			t.Fatalf("ParseDisconnect: %v", err)
		}
		if got != want {
			t.Errorf("came back %+v, want %+v", got, want)
		}
	}
}

func TestRejectingSomethingNotUnderstood(t *testing.T) {
	c := Reject(4, RejectBadCID, 0x0040, 0x0041)

	if c.Code != CodeReject || c.ID != 4 {
		t.Errorf("came out as %#x id %d", c.Code, c.ID)
	}
	if len(c.Data) != 6 {
		t.Fatalf("carried %d bytes, want the reason and two channels", len(c.Data))
	}
	if got := uint16(c.Data[0]) | uint16(c.Data[1])<<8; got != RejectBadCID {
		t.Errorf("reason came out %#x", got)
	}
}

func TestShortCommandBodies(t *testing.T) {
	for _, tc := range []struct {
		name  string
		parse func(Command) error
	}{
		{"connect", func(c Command) error { _, err := ParseConnect(c); return err }},
		{"connected", func(c Command) error { _, err := ParseConnected(c); return err }},
		{"disconnect", func(c Command) error { _, err := ParseDisconnect(c); return err }},
		{"configure", func(c Command) error { _, err := ParseConfigure(c); return err }},
		{"configured", func(c Command) error { _, err := ParseConfigured(c); return err }},
	} {
		if err := tc.parse(Command{Data: []byte{1}}); !errors.Is(err, ErrShort) {
			t.Errorf("%s: a one byte body gave %v, want a short read", tc.name, err)
		}
	}
}
