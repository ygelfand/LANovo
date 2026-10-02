package ble

import "testing"

// Taking whole packets off a stream of bytes.
//
// Runs without a UART on purpose: a packet split across reads, rubbish in front of one, and a sleep
// byte between two are the cases that matter, and the device is the least convenient place to find
// out any of them was handled wrongly.

// Half a packet is not a fault, it is the ordinary state of a line being read.
func TestDrainLeavesAPartialPacketAlone(t *testing.T) {
	s := &stream{held: []byte{typeEvent, 0x01, 0x04, 0xaa}}

	if s.drain(nil) {
		t.Error("half a packet was handed over")
	}
	if len(s.held) != 4 {
		t.Errorf("%d bytes left, want the partial packet kept", len(s.held))
	}

	// The rest arrives and it comes together.
	s.held = append(s.held, 0xbb, 0xcc, 0xdd)
	if !s.drain(nil) {
		t.Fatal("the completed packet was not found")
	}
	if got, _ := s.take(); string(got.Params) != "\xaa\xbb\xcc\xdd" {
		t.Errorf("params % x", got.Params)
	}
}

// Rubbish in front of a packet is stepped over rather than taking the packet with it.
func TestDrainStepsPastRubbish(t *testing.T) {
	s := &stream{held: []byte{0xaa, 0xbb, typeEvent, 0x01, 0x01, 0xcc}}

	if !s.drain(nil) {
		t.Fatal("the packet behind the rubbish was lost")
	}
	if got, _ := s.take(); got.Code != 0x01 || string(got.Params) != "\xcc" {
		t.Errorf("came back %#02x % x", got.Code, got.Params)
	}
}

// Nothing recognisable leaves nothing behind, rather than the same rubbish being rescanned for ever.
func TestDrainOnNothingButRubbish(t *testing.T) {
	s := &stream{held: []byte{0xaa, 0xbb, 0xcc}}

	if s.drain(nil) {
		t.Error("rubbish was read as a packet")
	}
	if len(s.held) != 0 {
		t.Errorf("% x was kept", s.held)
	}
}

// The sleep protocol is single bytes between packets, so it gets the front of the buffer before
// anything tries to parse it as one.
func TestDrainLetsTheSleepProtocolGoFirst(t *testing.T) {
	s := &stream{held: []byte{ibsSleep, typeEvent, 0x01, 0x00}}

	skip := func() bool {
		if len(s.held) > 0 && s.held[0] == ibsSleep {
			s.held = s.held[1:]
			return true
		}
		return false
	}

	if !s.drain(skip) {
		t.Fatal("the packet behind the sleep byte was lost")
	}
	if got, _ := s.take(); got.Code != 0x01 {
		t.Errorf("came back %#02x", got.Code)
	}
}

// Everything on the line is taken, not just the first one, and in the order it arrived: a command
// status and the event that followed it are not interchangeable.
func TestDrainTakesEverythingInOrder(t *testing.T) {
	s := &stream{held: []byte{
		typeEvent, 0x10, 0x00,
		typeEvent, 0x11, 0x00,
		typeEvent, 0x12, 0x00,
	}}

	if !s.drain(nil) {
		t.Fatal("nothing was found")
	}
	if len(s.held) != 0 {
		t.Errorf("% x was left on the line", s.held)
	}

	for _, want := range []byte{0x10, 0x11, 0x12} {
		got, ok := s.take()
		if !ok || got.Code != want {
			t.Errorf("came back %#02x %v, want %#02x", got.Code, ok, want)
		}
	}
}
