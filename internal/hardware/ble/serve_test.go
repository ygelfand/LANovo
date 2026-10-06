//go:build linux

package ble

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ygelfand/libcountertop/pkg/bluetooth/pair"
)

// The loop that carries a link, driven over a socketpair with no controller behind it.
//
// What it has to get right is all bookkeeping: a PDU split into fragments arrives as one, a reply
// goes out as fragments the controller will take, buffers are not overrun, and a link ending lets
// go of what was kept for it. None of that needs a radio, and none of it is visible from above.

// deciding is a Classic that records what it was asked and says what it was told to.
type deciding struct {
	mu sync.Mutex

	events []pair.Event
	pdus   [][]byte
	gone   []uint16

	// reply is what Data answers with, if anything.
	reply [][]byte

	// commands is what Event answers with, if anything.
	commands []pair.Command
}

func (d *deciding) Event(e pair.Event) ([]pair.Command, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.events = append(d.events, e)
	return d.commands, nil
}

func (d *deciding) Data(handle uint16, pdu []byte) ([][]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pdus = append(d.pdus, append([]byte(nil), pdu...))
	return d.reply, nil
}

func (d *deciding) Gone(handle uint16) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.gone = append(d.gone, handle)
}

func (d *deciding) saw() (int, int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.events), len(d.pdus), len(d.gone)
}

// aclOf is one ACL packet on a handle, carrying body.
func aclOf(h Handle, boundary byte, body []byte) []byte {
	out := []byte{typeACL}
	out = binary.LittleEndian.AppendUint16(out, uint16(h)|uint16(boundary)<<12)
	out = binary.LittleEndian.AppendUint16(out, uint16(len(body)))
	return append(out, body...)
}

// bufferSize is the controller answering Read Buffer Size: status, ACL length, SCO length, ACL
// count, SCO count.
func bufferSize(size, total int) []byte {
	out := []byte{typeEvent, eventCommandComplete, 0x0b, 0x01}
	out = binary.LittleEndian.AppendUint16(out, hciReadBufferSize)
	out = append(out, 0x00)
	out = binary.LittleEndian.AppendUint16(out, uint16(size))
	out = append(out, 0x00)
	out = binary.LittleEndian.AppendUint16(out, uint16(total))
	return binary.LittleEndian.AppendUint16(out, 0)
}

// serving starts the loop against a socketpair and returns what it decides with, and a stop.
func serving(t *testing.T, d *deciding, size, total int) (chip *os.File, stop func()) {
	t.Helper()

	p, chip := paired(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		Serve(ctx, Reader(ctx, p), d)
	}()

	// The loop asks what the controller holds before anything else, and a controller answers when
	// asked rather than beforehand. Answering early raced the reader: the event can arrive before
	// anything is waiting on it.
	asked(t, chip, hciReadBufferSize)
	if _, err := chip.Write(bufferSize(size, total)); err != nil {
		t.Fatal(err)
	}

	return chip, func() { cancel(); <-done }
}

// asked waits for one command to arrive on the chip side and checks it is the one expected.
func asked(t *testing.T, chip *os.File, opcode uint16) {
	t.Helper()

	want, err := command(opcode)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]byte, len(want))
	if err := chip.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(chip, got); err != nil {
		t.Fatalf("waiting for %#04x: %v", opcode, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("the loop sent % x, want % x", got, want)
	}

	if err := chip.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
}

func TestALinkCarriesAWholePDUFromItsFragments(t *testing.T) {
	d := &deciding{}
	chip, stop := serving(t, d, 64, 4)
	defer stop()

	// One 40 byte PDU: an L2CAP header saying 36 bytes of payload on channel 0x40, then the
	// payload. How long the whole message is only the PDU says — a fragment says how long the
	// fragment is.
	body := append([]byte{0x24, 0x00, 0x40, 0x00}, make([]byte, 36)...)
	for i := range body[4:] {
		body[4+i] = byte(i)
	}

	chip.Write(aclOf(1, boundaryFlushable, body[:20]))
	chip.Write(aclOf(1, boundaryContinue, body[20:]))

	settles(t, func() bool { _, pdus, _ := d.saw(); return pdus == 1 })

	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.pdus[0]) != len(body) {
		t.Fatalf("a PDU of %d bytes came back as %d", len(body), len(d.pdus[0]))
	}
	for i := range body {
		if d.pdus[0][i] != body[i] {
			t.Fatalf("byte %d is %#02x, want %#02x", i, d.pdus[0][i], body[i])
		}
	}
}

func TestAnEventReachesWhoeverDecides(t *testing.T) {
	d := &deciding{}
	chip, stop := serving(t, d, 64, 4)
	defer stop()

	chip.Write([]byte{typeEvent, pair.EventConnectionRequest, 0x03, 0xaa, 0xbb, 0xcc})

	settles(t, func() bool { events, _, _ := d.saw(); return events >= 1 })

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.events[0].Code != pair.EventConnectionRequest {
		t.Errorf("event %#02x reached the decider", d.events[0].Code)
	}
}

// A link ending both tells the decider and drops what was being reassembled for it, or half a PDU
// from a phone that has gone waits for a continuation that never comes.
func TestALinkEndingIsPassedOnAndForgotten(t *testing.T) {
	d := &deciding{}
	chip, stop := serving(t, d, 64, 4)
	defer stop()

	// A whole fragment carrying half a PDU: the header says 36 bytes of payload and only 16 are
	// here, so it waits. Then the link goes.
	half := append([]byte{0x24, 0x00, 0x40, 0x00}, make([]byte, 16)...)
	chip.Write(aclOf(7, boundaryFlushable, half))

	// Disconnection complete: status, handle, reason.
	chip.Write([]byte{typeEvent, pair.EventDisconnectionComplete, 0x04, 0x00, 0x07, 0x00, 0x13})

	settles(t, func() bool { _, _, gone := d.saw(); return gone == 1 })

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.gone[0] != 7 {
		t.Errorf("handle %d went, want 7", d.gone[0])
	}
}

// The controller holds a fixed number of packets. Writing past that is what makes one stop
// answering rather than refuse, so a reply longer than the buffers waits for them to come back.
func TestSendingWaitsForTheControllerToCatchUp(t *testing.T) {
	// 200 bytes over 32 byte packets is seven fragments, against two buffers.
	d := &deciding{reply: [][]byte{make([]byte, 200)}}

	chip, stop := serving(t, d, 32, 2)
	defer stop()

	var sent counter
	go io.Copy(&sent, chip)

	chip.Write(aclOf(1, boundaryFlushable, []byte{0x00, 0x00, 0x40, 0x00}))
	settles(t, func() bool { _, pdus, _ := d.saw(); return pdus == 1 })

	// Two buffers, so two fragments go and the rest wait. Nothing else can arrive until some come
	// back, which is the whole point.
	settles(t, func() bool { return sent.n() >= 2 })

	before := sent.n()
	time.Sleep(50 * time.Millisecond)

	if now := sent.n(); now != before {
		t.Errorf("%d packets went out with no buffers free, was %d", now, before)
	}

	// Hand back exactly what the remaining five fragments need, one at a time.
	for range 5 {
		chip.Write([]byte{typeEvent, eventCompletedPackets, 0x05, 0x01, 0x01, 0x00, 0x01, 0x00})
		time.Sleep(5 * time.Millisecond)
	}

	settles(t, func() bool { return sent.n() >= 7 })
}

// counter counts the ACL packets going out.
//
// By framing rather than by bytes, since a socket hands over whatever has arrived rather than one
// packet at a time — and it has to step over the commands on the same line, which are a different
// shape and would otherwise be read as data.
type counter struct {
	mu   sync.Mutex
	seen int
	held []byte
}

func (c *counter) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.held = append(c.held, b...)

	for len(c.held) > 0 {
		var n int

		switch c.held[0] {
		case typeACL:
			if len(c.held) < aclHeader {
				return len(b), nil
			}
			n = aclHeader + int(binary.LittleEndian.Uint16(c.held[3:]))

		case typeCommand:
			// Type, opcode, length.
			const header = 4
			if len(c.held) < header {
				return len(b), nil
			}
			n = header + int(c.held[3])

		default:
			// A sleep byte or something unrecognised: step over it rather than stalling.
			c.held = c.held[1:]
			continue
		}

		if len(c.held) < n {
			return len(b), nil
		}
		if c.held[0] == typeACL {
			c.seen++
		}
		c.held = c.held[n:]
	}
	return len(b), nil
}

func (c *counter) n() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seen
}

// settles blocks until want is true, or fails the test.
func settles(t *testing.T, want func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if want() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out")
}
