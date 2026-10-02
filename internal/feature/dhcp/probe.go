package dhcp

import (
	"encoding/binary"
	"net"
	"time"
)

// Probe asks whether anything on the segment already holds an address, which is what the client
// does before it takes a new one.
//
// Exported for lanovod tools probe. Running it by hand against an address that is in use is the
// only way to see the conflict path work on real hardware: the client says nothing when the
// address is free, which is the case on every network it is likely to be tried on.
func Probe(iface string, ip net.IP) (bool, error) { return probe(iface, ip) }

// What RFC 5227 section 2.1 asks for: a wait of up to a second, then three probes one to two
// seconds apart, and no use of the address until the last one has gone unanswered.
const (
	probeCount = 3
	probeWait  = time.Second
	probeMin   = time.Second
	probeMax   = 2 * time.Second
)

// arpProbe builds a probe: an ARP request for an address, from a host that does not have one.
//
// The sender protocol address is zero, and that is what makes it a probe rather than an ordinary
// question. A host that hears one does not record the sender, so asking about an address somebody
// else holds cannot itself corrupt their caches with a claim to it.
func arpProbe(mac net.HardwareAddr, ip net.IP) []byte {
	ip = ip.To4()
	if ip == nil || len(mac) != 6 {
		return nil
	}

	frame := make([]byte, ethHeader+arpPayload)

	copy(frame[0:6], broadcast)
	copy(frame[6:12], mac)
	binary.BigEndian.PutUint16(frame[12:14], 0x0806) // ARP

	arp := frame[ethHeader:]
	binary.BigEndian.PutUint16(arp[0:2], 1)      // ethernet
	binary.BigEndian.PutUint16(arp[2:4], 0x0800) // IPv4
	arp[4] = 6                                   // hardware address length
	arp[5] = 4                                   // protocol address length
	binary.BigEndian.PutUint16(arp[6:8], 1)      // request

	copy(arp[8:14], mac) // sender hardware address
	copy(arp[24:28], ip) // target protocol address, the one being asked about

	// The sender protocol address stays zero, and so does the target hardware address.
	return frame
}

// taken reports whether a frame says somebody else already holds want.
//
// Two shapes count, both from RFC 5227 section 2.1. An ARP packet whose sender protocol address is
// the address being probed is a host using it. And a probe from another host for the same address
// — sender zero, target ours — is a second machine about to take it, which is a conflict before
// either of them has it.
//
// A frame this device sent is not a conflict: the socket hears its own probes back.
func taken(frame []byte, mine net.HardwareAddr, want net.IP) bool {
	want = want.To4()
	if want == nil {
		return false
	}

	got, ok := readARP(frame)
	if !ok || got.ours(mine) {
		return false
	}

	if got.senderIP.Equal(want) {
		return true
	}
	return got.senderIP.Equal(net.IPv4zero) && got.targetIP.Equal(want)
}
