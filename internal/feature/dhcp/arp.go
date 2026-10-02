package dhcp

import (
	"encoding/binary"
	"net"
)

// arpFrame is the part of an ARP frame anything here cares about.
//
// Shared because two questions are asked of the same bytes — whether somebody already holds an
// address we are about to take, and whether somebody is claiming one we already hold — and the
// header checks in front of both are the same. Two copies of them is two places for a length check
// to be forgotten.
type arpFrame struct {
	op uint16

	senderMAC net.HardwareAddr
	senderIP  net.IP
	targetIP  net.IP
}

// ARP operations. A reply is as much of a claim as a request: what matters is the sender fields,
// not which way round the packet is.
const (
	arpRequest = 1
	arpReply   = 2
)

// readARP pulls an ARP frame apart, reporting false for anything that is not one.
//
// Ethernet and IPv4 only, with the lengths the two of them imply. Anything else on the wire is not
// something this device has an opinion about, and reading it as though it were would take the
// sender address out of the middle of some other protocol.
func readARP(frame []byte) (arpFrame, bool) {
	if len(frame) < ethHeader+arpPayload {
		return arpFrame{}, false
	}
	if binary.BigEndian.Uint16(frame[12:14]) != 0x0806 {
		return arpFrame{}, false
	}

	body := frame[ethHeader:]
	if binary.BigEndian.Uint16(body[0:2]) != 1 || binary.BigEndian.Uint16(body[2:4]) != 0x0800 {
		return arpFrame{}, false
	}
	if body[4] != 6 || body[5] != 4 {
		return arpFrame{}, false
	}

	op := binary.BigEndian.Uint16(body[6:8])
	if op != arpRequest && op != arpReply {
		return arpFrame{}, false
	}

	return arpFrame{
		op:        op,
		senderMAC: net.HardwareAddr(body[8:14]),
		senderIP:  net.IP(body[14:18]),
		targetIP:  net.IP(body[24:28]),
	}, true
}

// ours reports whether this device sent the frame. A packet socket hears its own traffic back, so
// every reader here has to be able to discount itself.
func (a arpFrame) ours(mine net.HardwareAddr) bool {
	return len(mine) == 6 && a.senderMAC.String() == mine.String()
}
