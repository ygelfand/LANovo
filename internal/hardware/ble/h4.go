// Package ble brings the Bluetooth radio up and speaks HCI to it.
//
// There is no hci interface on this device and there cannot be one: the kernel has CONFIG_BT but
// not CONFIG_BT_HCIUART, so nothing attaches the chip's UART to the Bluetooth core. Android does
// the same thing from userspace through a vendor service, which downloads firmware to the chip and
// then speaks HCI down the tty. This is that, in Go.
//
// The chip is a QCA9379 — the device tree calls the node bt_qca9379 and the power driver claims
// qca,qca6174 — which is ROME generation, so the bring-up is ROME's: read the version over a
// vendor command, push a TLV patch and an NVM blob in segments, raise the baud rate, and from
// there it is ordinary HCI.
package ble

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// H4 is the framing UART HCI uses: one byte saying what the packet is, then the packet.
const (
	typeCommand = 0x01
	typeACL     = 0x02
	typeSCO     = 0x03
	typeEvent   = 0x04
)

const (
	scoHeader = 4
	scoKept   = 64
)

// The chip's in-band sleep protocol, which the firmware turns on the moment it starts running and
// the loader knows nothing about. Single bytes, outside H4 framing: no type, no length.
const (
	ibsSleep   = 0xfe
	ibsWake    = 0xfd
	ibsWakeAck = 0xfc
)

// The events a bring-up reads. Everything else on the wire is somebody else's.
const (
	eventCommandComplete = 0x0e
	eventCommandStatus   = 0x0f
	eventVendor          = 0xff
)

// The ordinary HCI commands the bring-up sends. Everything else it says is a vendor command.
const (
	hciReset             = 0x0c03
	hciReadLocalVersion  = 0x1001
	hciReadLocalFeatures = 0x1003
)

// wakeAgain is how often the wake byte is repeated while waiting to be acknowledged.
//
// A chip in deep sleep spends the first byte waking its own UART and never sees it as a wake, so a
// single one is a handshake that is lost about as often as it lands.
const wakeAgain = 100 * time.Millisecond

// A command is at most 255 bytes of parameters, which is what the one length byte holds.
const maxParams = 255

var (
	errShort = errors.New("ble: the packet is shorter than its header says")

	// errQuiet is nothing having arrived, which is a fault while waiting on an answer and the
	// normal state of the line while scanning.
	errQuiet = errors.New("ble: nothing came back")
)

// command frames an HCI command for the wire: H4 type, opcode, length, parameters.
func command(opcode uint16, params ...byte) ([]byte, error) {
	if len(params) > maxParams {
		return nil, fmt.Errorf("ble: %d parameters, more than the %d a command carries",
			len(params), maxParams)
	}

	out := make([]byte, 4+len(params))
	out[0] = typeCommand
	binary.LittleEndian.PutUint16(out[1:], opcode)
	out[3] = byte(len(params))
	copy(out[4:], params)
	return out, nil
}

// event is one HCI event read back off the wire.
type event struct {
	Code   byte
	Params []byte
}

// packet is one thing off the line. The type byte says which of the two it is.
type packet struct {
	kind  byte
	event event
	acl   aclPacket
	sco   scoPacket
}

type scoPacket struct {
	Handle Handle
	Status byte
	Data   []byte
}

// pending is what has been read while nobody was waiting for it.
//
// Two queues, because a live link carries both at once and they are wanted by different readers: a
// command's answer arrives between two packets of audio, and whichever the caller was not waiting
// for has to survive until somebody asks. Dropping an event loses an answer; dropping ACL loses
// music.
//
// A read for one also turns up more of its own — an advertising report in the middle of waiting for
// a command complete — which is the other half of why these are kept rather than discarded.
type pending struct {
	events []event
	acl    []aclPacket
	sco    []scoPacket
	scos   int
}

func (q *pending) add(p packet) {
	switch p.kind {
	case typeACL:
		q.acl = append(q.acl, p.acl)
	case typeSCO:
		q.scos++
		if len(q.sco) == scoKept {
			q.sco = append(q.sco[:0], q.sco[1:]...)
		}
		q.sco = append(q.sco, p.sco)
	default:
		q.events = append(q.events, p.event)
	}
}

// take is the oldest event waiting for a reader, and whether there was one. In order, since a
// command status and the event that followed it are not interchangeable.
func (q *pending) take() (event, bool) {
	if len(q.events) == 0 {
		return event{}, false
	}

	e := q.events[0]
	q.events = q.events[1:]
	return e, true
}

// takeACL is the same for data, which is kept in order for the reassembler's sake: a continuation
// read before the packet it continues is a message that never comes back together.
func (q *pending) takeACL() (aclPacket, bool) {
	if len(q.acl) == 0 {
		return aclPacket{}, false
	}

	a := q.acl[0]
	q.acl = q.acl[1:]
	return a, true
}

// stream is what has been read off the line and not yet handed to a reader.
//
// Kept apart from the port so the framing can be driven without a UART, which is the part a device
// is least convenient for testing.
type stream struct {
	pending

	// held is read and not yet parsed into a packet.
	held []byte
}

// drain takes whole packets off the front of what has been read and files them, reporting whether
// it filed any.
//
// skip is the sleep protocol, which is single bytes between packets rather than packets, so it gets
// first refusal on the front of the buffer. Nil for a caller that has none.
func (s *stream) drain(skip func() bool) bool {
	var any bool

	for len(s.held) > 0 {
		if skip != nil && skip() {
			continue
		}

		got, took, err := parsePacket(s.held)
		if errors.Is(err, errShort) {
			// The rest of this packet has not arrived. Not a fault, just not yet.
			break
		}
		if err != nil {
			s.held = resync(s.held)
			continue
		}

		s.held = s.held[took:]
		s.add(got)
		any = true
	}

	return any
}

// resync drops whatever is in front of the next packet of either kind.
//
// A last resort: every byte the chip sends is accounted for, so reaching this means the reader is
// mid-packet. It hides a desync rather than explaining one — and it has to know both types, or it
// walks past the start of an ACL packet looking for an event and takes the rest with it.
func resync(buf []byte) []byte {
	for i, b := range buf {
		if b == typeEvent || b == typeACL || b == typeSCO {
			return buf[i:]
		}
	}
	return nil
}

// parsePacket reads one event from the front of buf and reports how much of buf it took.
//
// Nothing is assumed about what else is in the buffer. A UART hands over whatever has arrived,
// which is a fraction of a packet as often as it is a whole one, so this says how far it got and
// leaves the caller to read more.
func parsePacket(buf []byte) (p packet, took int, err error) {
	if len(buf) == 0 {
		return packet{}, 0, errShort
	}

	if buf[0] == typeACL {
		a, n, err := parseACL(buf)
		if err != nil {
			return packet{}, 0, err
		}
		return packet{kind: typeACL, acl: a}, n, nil
	}
	if buf[0] == typeSCO {
		if len(buf) < scoHeader {
			return packet{}, 0, errShort
		}
		n := int(buf[3])
		if len(buf) < scoHeader+n {
			return packet{}, 0, errShort
		}
		word := binary.LittleEndian.Uint16(buf[1:])
		return packet{kind: typeSCO, sco: scoPacket{
			Handle: Handle(word & handleMask),
			Status: byte(word >> 12 & 0x3),
			Data:   buf[scoHeader : scoHeader+n],
		}}, scoHeader + n, nil
	}

	// Type, code, length.
	const header = 3

	if len(buf) < header {
		return packet{}, 0, errShort
	}
	if buf[0] != typeEvent {
		return packet{}, 0, fmt.Errorf("ble: packet type %#02x is not an event, acl or sco", buf[0])
	}

	n := int(buf[2])
	if len(buf) < header+n {
		return packet{}, 0, errShort
	}
	return packet{kind: typeEvent, event: event{Code: buf[1], Params: buf[header : header+n]}},
		header + n, nil
}

// complete reads a Command Complete for the opcode expected, and returns what came back after the
// status byte.
//
// The count of allowed commands is skipped: it is flow control for a host queueing several at
// once, and this queues one and waits.
func complete(e event, opcode uint16) ([]byte, error) {
	if e.Code != eventCommandComplete {
		return nil, fmt.Errorf("ble: event %#02x, want a command complete", e.Code)
	}
	if len(e.Params) < 4 {
		return nil, errShort
	}

	if got := binary.LittleEndian.Uint16(e.Params[1:]); got != opcode {
		return nil, fmt.Errorf("ble: a command complete for %#04x, waiting on %#04x", got, opcode)
	}
	if status := e.Params[3]; status != 0 {
		return nil, fmt.Errorf("ble: %#04x refused, status %#02x", opcode, status)
	}
	return e.Params[4:], nil
}
