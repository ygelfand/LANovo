//go:build linux

package ble

import (
	"errors"
	"os"
	"testing"
	"time"
)

// Reading, driven through a real read rather than through its parts.
//
// parsePacket and pending are each checked on their own, and the loop that joins them is where a
// mistake would actually live: an event dropped between being parsed and being put aside, or a
// packet that arrived in two pieces read as one short one. A pipe is a real *os.File with a working
// deadline, which is all the read loop touches, so the whole thing runs without hardware.

// wired is a port reading from a pipe, and the end to write packets into.
func wired(t *testing.T) (*Port, *os.File) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	return &Port{f: r}, w
}

const soon = 2 * time.Second

// A UART splits packets where it likes, so half of one is the ordinary case rather than a fault.
func TestAPacketSplitAcrossReads(t *testing.T) {
	p, w := wired(t)

	w.Write([]byte{typeEvent, 0x0e, 0x02})
	go func() {
		time.Sleep(20 * time.Millisecond)
		w.Write([]byte{0xbb, 0xcc})
	}()

	got, err := p.Read(soon)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Code != 0x0e || string(got.Params) != "\xbb\xcc" {
		t.Errorf("came back %#02x % x", got.Code, got.Params)
	}
}

// Everything on the line is taken in one go, and comes back in the order it arrived.
func TestQueuedEventsKeepTheirOrder(t *testing.T) {
	p, w := wired(t)

	w.Write([]byte{
		typeEvent, 0x10, 0x00,
		typeEvent, 0x11, 0x00,
		typeEvent, 0x12, 0x00,
	})

	for _, want := range []byte{0x10, 0x11, 0x12} {
		got, err := p.Read(soon)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if got.Code != want {
			t.Errorf("came back %#02x, want %#02x", got.Code, want)
		}
	}
}

// A line with nothing on it reports quiet rather than hanging.
func TestAQuietLine(t *testing.T) {
	p, _ := wired(t)

	if _, err := p.Read(50 * time.Millisecond); !errors.Is(err, errQuiet) {
		t.Errorf("gave %v, want quiet", err)
	}
}

// Rubbish in front of a packet is stepped over rather than taking the packet with it.
func TestReadingPastRubbishOnTheLine(t *testing.T) {
	p, w := wired(t)

	w.Write([]byte{0xaa, 0xbb, typeEvent, 0x01, 0x01, 0xcc})

	got, err := p.Read(soon)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Code != 0x01 || string(got.Params) != "\xcc" {
		t.Errorf("came back %#02x % x", got.Code, got.Params)
	}
}
