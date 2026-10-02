package dhcp

import (
	"encoding/binary"
	"net"
)

// broadcast is who an announcement is addressed to: everything on the segment.
var broadcast = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// The sizes of what an announcement is made of.
const (
	ethHeader  = 14
	arpPayload = 28
)

// arpAnnounce builds the frame: an ARP request in which the device is both the sender and the
// thing being asked about.
//
// Asking about itself is what makes it an announcement rather than a question. A host that hears
// one records the sender, and nobody answers, because the only machine that could is the one that
// asked.
func arpAnnounce(mac net.HardwareAddr, ip net.IP) []byte {
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
	copy(arp[14:18], ip) // sender protocol address
	copy(arp[24:28], ip) // target protocol address, the same

	// The target hardware address stays zero: it is the thing an answer would carry.
	return frame
}
