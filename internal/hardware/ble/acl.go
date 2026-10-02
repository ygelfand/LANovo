package ble

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
)

// ACL is Bluetooth's data channel, as opposed to the commands and events everything else here
// speaks. A scan never needs it — advertising arrives as events — but nothing above it can exist
// without it: L2CAP, SDP, AVDTP and the audio itself are all bytes inside ACL packets.
//
// Three things live here and nothing else does. Framing, because a packet carries a connection
// handle and flags rather than an opcode. Fragmentation, because the controller has a maximum
// payload and a larger message is split across packets that the far end puts back together.
// And credit, because the controller has a fixed number of buffers and a host that writes past
// them is not told off, it is dropped.

// Handle identifies one connection. Twelve bits, so the top four of the word carry flags instead.
type Handle uint16

const handleMask = 0x0fff

// Where a packet sits in a message that did not fit in one.
//
// Start and Flushable both mean "first fragment", and which one is used says who sent it: a host
// says non-automatically-flushable, a controller says automatically-flushable. Reading either as
// the beginning is right; writing the wrong one is a packet the controller may flush when it
// should not.
const (
	boundaryStart     = 0x00 // host to controller, first fragment
	boundaryContinue  = 0x01
	boundaryFlushable = 0x02 // controller to host, first fragment
	boundaryComplete  = 0x03 // a whole PDU in one packet, LE only
)

// aclHeader is the H4 type byte, the handle and flags word, and the length word.
const aclHeader = 5

// l2capHeader is the length and channel id every PDU starts with. Reassembly needs it: a fragment
// says how long it is, and only the PDU inside says how long the whole message is.
const l2capHeader = 4

// aclPacket is one HCI ACL data packet.
type aclPacket struct {
	Handle    Handle
	Boundary  byte
	Broadcast byte
	Data      []byte
}

// starts reports whether this packet begins a message rather than continuing one.
func (p aclPacket) starts() bool {
	return p.Boundary == boundaryStart || p.Boundary == boundaryFlushable ||
		p.Boundary == boundaryComplete
}

// parseACL reads one ACL packet from the front of buf and reports how much of buf it took.
//
// Like parseEvent: a UART hands over whatever has arrived, so a short buffer is normal and says
// read more rather than something is wrong.
func parseACL(buf []byte) (aclPacket, int, error) {
	if len(buf) < aclHeader {
		return aclPacket{}, 0, errShort
	}
	if buf[0] != typeACL {
		return aclPacket{}, 0, fmt.Errorf("ble: packet type %#02x is not acl", buf[0])
	}

	word := binary.LittleEndian.Uint16(buf[1:])
	n := int(binary.LittleEndian.Uint16(buf[3:]))

	if len(buf) < aclHeader+n {
		return aclPacket{}, 0, errShort
	}

	return aclPacket{
		Handle:    Handle(word & handleMask),
		Boundary:  byte(word >> 12 & 0x3),
		Broadcast: byte(word >> 14 & 0x3),
		Data:      buf[aclHeader : aclHeader+n],
	}, aclHeader + n, nil
}

// aclFrames cuts one L2CAP PDU into packets the controller will take.
//
// mtu is the controller's ACL data packet length, which is payload and does not count the header.
// The first fragment says it starts a message and the rest say they continue one; getting that
// wrong gives the far end two half messages rather than one whole one.
func aclFrames(h Handle, pdu []byte, mtu int) ([][]byte, error) {
	switch {
	case len(pdu) == 0:
		return nil, fmt.Errorf("ble: nothing to send on handle %#x", h)
	case mtu <= 0:
		return nil, fmt.Errorf("ble: the controller takes %d bytes a packet", mtu)
	case h&^handleMask != 0:
		return nil, fmt.Errorf("ble: %#x is more than a twelve bit handle", h)
	}

	var out [][]byte
	boundary := byte(boundaryStart)

	for len(pdu) > 0 {
		n := min(mtu, len(pdu))

		frame := make([]byte, aclHeader+n)
		frame[0] = typeACL
		binary.LittleEndian.PutUint16(frame[1:], uint16(h)|uint16(boundary)<<12)
		binary.LittleEndian.PutUint16(frame[3:], uint16(n))
		copy(frame[aclHeader:], pdu[:n])

		out = append(out, frame)
		pdu = pdu[n:]
		boundary = boundaryContinue
	}
	return out, nil
}

// reassembler puts fragments back into whole L2CAP PDUs, one message at a time per connection.
//
// It has to read the L2CAP length to know when a message is done, because an ACL packet only says
// how long itself is. That is the one place this layer looks at what it is carrying, and it is
// unavoidable: HCI has no length of its own for the whole message.
type reassembler struct {
	parts map[Handle][]byte
}

func newReassembler() *reassembler { return &reassembler{parts: map[Handle][]byte{}} }

// add takes one packet and answers with a whole PDU once it has one.
//
// A fragment that continues a message nobody started, or a message that starts while another is
// half read, is a controller not following the protocol rather than a short read. Both are errors:
// carrying on would hand L2CAP a PDU stitched out of two messages.
func (r *reassembler) add(p aclPacket) ([]byte, error) {
	held, going := r.parts[p.Handle]

	switch {
	case p.starts() && going:
		delete(r.parts, p.Handle)
		return nil, fmt.Errorf("ble: handle %#x started a message with %d bytes of one outstanding",
			p.Handle, len(held))

	case !p.starts() && !going:
		return nil, fmt.Errorf("ble: handle %#x continued a message that never started", p.Handle)
	}

	buf := append(held, p.Data...)

	// Not enough to know how long the message is yet. A first fragment shorter than the L2CAP
	// header is legal and rare.
	if len(buf) < l2capHeader {
		r.parts[p.Handle] = buf
		return nil, nil
	}

	want := l2capHeader + int(binary.LittleEndian.Uint16(buf))
	switch {
	case len(buf) < want:
		r.parts[p.Handle] = buf
		return nil, nil

	case len(buf) > want:
		delete(r.parts, p.Handle)
		return nil, fmt.Errorf("ble: handle %#x sent %d bytes for a %d byte pdu",
			p.Handle, len(buf), want)
	}

	delete(r.parts, p.Handle)
	return buf, nil
}

// forget drops whatever a connection had half read, for a link that has gone away.
func (r *reassembler) forget(h Handle) { delete(r.parts, h) }

// pending is how many connections have a part-read message, for a caller that wants to say so.
func (r *reassembler) pending() int { return len(r.parts) }

// credit is the controller's ACL buffers, counted.
//
// The controller says how many packets it can hold and gives them back as it sends them, in a
// Number Of Completed Packets event. A host that writes more than it has been given is not
// refused; the packets are dropped, or the controller stops, and neither says why. So this is
// counted rather than hoped about.
type credit struct {
	mu   sync.Mutex
	more chan struct{}

	// size is the largest payload a packet carries and total is how many the controller holds.
	size  int
	total int
	free  int
}

func newCredit(size, total int) (*credit, error) {
	if size <= 0 || total <= 0 {
		return nil, fmt.Errorf("ble: the controller reported %d buffers of %d bytes", total, size)
	}
	return &credit{more: make(chan struct{}, 1), size: size, total: total, free: total}, nil
}

// Size is the largest payload one packet carries, which is what fragmentation is measured against.
func (c *credit) Size() int { return c.size }

// take claims one buffer, waiting until the controller gives one back if there are none.
func (c *credit) take(ctx context.Context) error {
	for {
		c.mu.Lock()
		if c.free > 0 {
			c.free--
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.more:
		}
	}
}

// give hands buffers back, which is what a Number Of Completed Packets event means.
//
// More than were taken is the controller miscounting, and believing it would let the host write
// past what the hardware holds — so the ceiling is kept rather than the claim.
func (c *credit) give(n int) error {
	c.mu.Lock()

	c.free += n
	over := c.free > c.total
	if over {
		c.free = c.total
	}
	c.mu.Unlock()

	// Buffered and non-blocking: a waiter that is already awake does not need telling twice.
	select {
	case c.more <- struct{}{}:
	default:
	}

	if over {
		return fmt.Errorf("ble: the controller gave back %d buffers, more than the %d it has",
			n, c.total)
	}
	return nil
}

// available is how many buffers are free, for a caller reporting on itself.
func (c *credit) available() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.free
}

// hciReadBufferSize asks the controller how much it can hold, and the event that hands buffers back
// as it empties them.
const (
	hciReadBufferSize     = 0x1005
	eventCompletedPackets = 0x13
)

// parseBufferSize reads a Read Buffer Size reply, from after the status byte — which is what
// complete hands back.
//
// The synchronous numbers beside them are for SCO, which is a phone call (#129) rather than music,
// and are read past rather than kept.
func parseBufferSize(params []byte) (size, total int, err error) {
	// ACL length, SCO length, ACL count, SCO count.
	const want = 2 + 1 + 2 + 2

	if len(params) < want {
		return 0, 0, fmt.Errorf("ble: %d bytes of buffer sizes, want %d", len(params), want)
	}

	size = int(binary.LittleEndian.Uint16(params[0:]))
	total = int(binary.LittleEndian.Uint16(params[3:]))

	if size == 0 || total == 0 {
		return 0, 0, fmt.Errorf("ble: the controller reported %d buffers of %d bytes", total, size)
	}
	return size, total, nil
}

// completed is one connection's worth of buffers coming back.
type completed struct {
	Handle Handle
	Count  int
}

// parseCompletedPackets reads a Number Of Completed Packets event.
//
// One event covers several connections, which is why it is a list rather than a pair: the
// controller batches whatever it finished since it last said so.
func parseCompletedPackets(e event) ([]completed, error) {
	if e.Code != eventCompletedPackets {
		return nil, fmt.Errorf("ble: event %#02x is not number of completed packets", e.Code)
	}
	if len(e.Params) < 1 {
		return nil, errShort
	}

	n := int(e.Params[0])
	rest := e.Params[1:]

	if len(rest) < n*4 {
		return nil, fmt.Errorf("ble: %d handles reported and %d bytes to say it in", n, len(rest))
	}

	out := make([]completed, 0, n)
	for i := range n {
		at := rest[i*4:]
		out = append(out, completed{
			Handle: Handle(binary.LittleEndian.Uint16(at) & handleMask),
			Count:  int(binary.LittleEndian.Uint16(at[2:])),
		})
	}
	return out, nil
}
