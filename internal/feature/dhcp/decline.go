package dhcp

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// The ports the two ends of DHCP speak on.
const (
	clientPort = 68
	serverPort = 67
)

// The headers a DHCP message travels under, both fixed length here: no IP options and no
// fragmentation.
const (
	ipHeader  = 20
	udpHeader = 8
)

// ttl is what the datagram is sent with. It is broadcast to 255.255.255.255 and no router forwards
// that, so this cannot matter; 64 is what everything else on the wire uses.
const ttl = 64

// declineMessage is the DHCPDECLINE itself, as RFC 2131 section 3.1.5 describes it: the address
// being refused in the requested-address option, and the server that offered it, so a network with
// two servers only hears from the one whose offer this was.
//
// The address goes in the option rather than in ciaddr. ciaddr is for an address the client is
// using, and the whole point of a decline is that it never started using this one.
func declineMessage(mac net.HardwareAddr, addr, server net.IP, why string) ([]byte, error) {
	if addr.To4() == nil {
		return nil, fmt.Errorf("dhcp: %s is not an IPv4 address", addr)
	}

	msg, err := dhcpv4.New(
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeDecline),
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(addr)),
		dhcpv4.WithOption(dhcpv4.OptMessage(why)),
	)
	if err != nil {
		return nil, fmt.Errorf("dhcp: building a decline: %w", err)
	}

	// Only when the server said who it was. A lease with no server identifier is one from a server
	// that did not name itself, and inventing one would address the decline to nobody.
	if server.To4() != nil {
		msg.UpdateOption(dhcpv4.OptServerIdentifier(server))
	}
	return msg.ToBytes(), nil
}

// declineFrame wraps a DHCP message in the UDP, IP and ethernet headers it needs to go out before
// the device has an address.
//
// By hand because there is nowhere else to put it. A socket bound to a port needs an address to
// bind to, which is exactly what a client declining an address does not have, and nclient4 builds
// the rest of the exchange but keeps its send path to itself and has no helper for a decline.
func declineFrame(mac net.HardwareAddr, payload []byte) []byte {
	if len(mac) != 6 {
		return nil
	}

	frame := make([]byte, ethHeader+ipHeader+udpHeader+len(payload))

	copy(frame[0:6], broadcast)
	copy(frame[6:12], mac)
	binary.BigEndian.PutUint16(frame[12:14], 0x0800) // IPv4

	ip := frame[ethHeader:]
	udp := ip[ipHeader:]
	copy(udp[udpHeader:], payload)

	from, to := net.IPv4zero.To4(), net.IPv4bcast.To4()

	ip[0] = 0x45 // version 4, five words of header
	binary.BigEndian.PutUint16(ip[2:4], uint16(ipHeader+udpHeader+len(payload)))
	// Identification stays zero: this datagram is never fragmented, so there is nothing for a
	// receiver to reassemble it with.
	ip[8] = ttl
	ip[9] = 17 // UDP
	copy(ip[12:16], from)
	copy(ip[16:20], to)
	binary.BigEndian.PutUint16(ip[10:12], checksum(ip[:ipHeader]))

	binary.BigEndian.PutUint16(udp[0:2], clientPort)
	binary.BigEndian.PutUint16(udp[2:4], serverPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpHeader+len(payload)))
	binary.BigEndian.PutUint16(udp[6:8], udpChecksum(from, to, udp))

	return frame
}

// udpChecksum is the sum over the pseudo header and the datagram, which is what makes a UDP
// checksum cover the addresses as well as the payload.
//
// Zero means no checksum in UDP, so a sum that comes out zero is sent as all ones instead. The two
// are the same number in ones complement and only one of them means "I did not check".
func udpChecksum(from, to net.IP, datagram []byte) uint16 {
	pseudo := make([]byte, 12)
	copy(pseudo[0:4], from)
	copy(pseudo[4:8], to)
	pseudo[9] = 17 // UDP
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(datagram)))

	if sum := checksum(pseudo, datagram); sum != 0 {
		return sum
	}
	return 0xffff
}

// checksum is the sixteen bit ones complement sum of ones complement sums from RFC 1071, which the
// IP header and the UDP datagram both use.
//
// The field being summed has to be zero while it is summed, which is why both callers fill it in
// afterwards.
func checksum(parts ...[]byte) uint16 {
	var sum uint32
	var odd bool

	for _, part := range parts {
		for i, b := range part {
			// Whether a byte is the high or low half follows the running length, not the index
			// within this part: two parts of odd length join in the middle of a word.
			if (i%2 == 0) != odd {
				sum += uint32(b) << 8
			} else {
				sum += uint32(b)
			}
		}
		if len(part)%2 == 1 {
			odd = !odd
		}
	}

	for sum>>16 != 0 {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}
