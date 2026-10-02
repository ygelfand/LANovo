package dhcp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

func declined(t *testing.T, addr, server net.IP) []byte {
	t.Helper()

	payload, err := declineMessage(testMAC, addr, server, "address already in use")
	if err != nil {
		t.Fatal(err)
	}

	frame := declineFrame(testMAC, payload)
	if frame == nil {
		t.Fatal("no frame was built")
	}
	return frame
}

// A header carrying its own correct checksum sums to zero. That is what a receiver does with it,
// and it is the only check that says the sum is right rather than merely consistent with itself.
func TestTheIPHeaderChecksumVerifies(t *testing.T) {
	ip := declined(t, net.IPv4(10, 100, 101, 106), net.IPv4(10, 100, 101, 1))[ethHeader:]

	if got := checksum(ip[:ipHeader]); got != 0 {
		t.Errorf("the header sums to %#04x, want 0 — a receiver would drop it", got)
	}
}

// The same for UDP, over the pseudo header that makes the sum cover the addresses too. Zero and
// all ones are the same number in ones complement, and a receiver accepts either.
func TestTheUDPChecksumVerifies(t *testing.T) {
	frame := declined(t, net.IPv4(10, 100, 101, 106), net.IPv4(10, 100, 101, 1))
	ip := frame[ethHeader:]
	udp := ip[ipHeader:]

	pseudo := make([]byte, 12)
	copy(pseudo[0:4], ip[12:16])
	copy(pseudo[4:8], ip[16:20])
	pseudo[9] = 17
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(udp)))

	if got := checksum(pseudo, udp); got != 0 && got != 0xffff {
		t.Errorf("the datagram sums to %#04x, want 0 — a receiver would drop it", got)
	}
}

// The sum is over a stream of bytes, not over each part separately: two parts of odd length join
// in the middle of a sixteen bit word, and a DHCP payload is whatever length its options make it.
func TestChecksumJoinsPartsOfOddLength(t *testing.T) {
	first := []byte{0x01, 0x02, 0x03}
	second := []byte{0x04, 0x05}

	whole := append(append([]byte{}, first...), second...)

	if a, b := checksum(first, second), checksum(whole); a != b {
		t.Errorf("summed in parts %#04x, summed whole %#04x", a, b)
	}
}

func TestChecksumOfAnEmptyThing(t *testing.T) {
	if got := checksum(); got != 0xffff {
		t.Errorf("checksum() = %#04x, want %#04x", got, 0xffff)
	}
}

// Every header, field by field. A frame that is almost right is one that goes out and is dropped
// by something that never says so.
func TestTheDeclineIsAddressedAsABroadcastFromNowhere(t *testing.T) {
	frame := declined(t, net.IPv4(10, 100, 101, 106), net.IPv4(10, 100, 101, 1))

	if !bytes.Equal(frame[0:6], broadcast) {
		t.Error("the frame is not broadcast, so a server that has not heard of us will not see it")
	}
	if !bytes.Equal(frame[6:12], testMAC) {
		t.Error("the frame does not come from this device")
	}
	if got := binary.BigEndian.Uint16(frame[12:14]); got != 0x0800 {
		t.Errorf("ethertype = %#04x, want IPv4", got)
	}

	ip := frame[ethHeader:]
	if ip[0] != 0x45 {
		t.Errorf("version and header length byte is %#02x, want 0x45", ip[0])
	}
	if ip[9] != 17 {
		t.Errorf("protocol = %d, want 17 for UDP", ip[9])
	}
	if got := net.IP(ip[12:16]); !got.Equal(net.IPv4zero) {
		t.Errorf("source = %v, want 0.0.0.0 — the client has no address, which is the point", got)
	}
	if got := net.IP(ip[16:20]); !got.Equal(net.IPv4bcast) {
		t.Errorf("destination = %v, want the broadcast address", got)
	}

	udp := ip[ipHeader:]
	if from := binary.BigEndian.Uint16(udp[0:2]); from != clientPort {
		t.Errorf("source port = %d, want %d", from, clientPort)
	}
	if to := binary.BigEndian.Uint16(udp[2:4]); to != serverPort {
		t.Errorf("destination port = %d, want %d", to, serverPort)
	}
}

// The lengths in the two headers have to agree with the frame, or the receiver reads past the end
// of the payload or stops short of it.
func TestTheLengthsAgreeWithTheFrame(t *testing.T) {
	frame := declined(t, net.IPv4(10, 100, 101, 106), net.IPv4(10, 100, 101, 1))
	ip := frame[ethHeader:]

	if got, want := int(binary.BigEndian.Uint16(ip[2:4])), len(frame)-ethHeader; got != want {
		t.Errorf("the IP header says %d bytes, the frame has %d", got, want)
	}
	if got, want := int(binary.BigEndian.Uint16(ip[ipHeader+4:ipHeader+6])), len(ip)-ipHeader; got != want {
		t.Errorf("the UDP header says %d bytes, the datagram has %d", got, want)
	}
}

// What the server is being told, read back the way it will read it.
func TestADeclineNamesTheAddressAndTheServer(t *testing.T) {
	addr, server := net.IPv4(10, 100, 101, 106), net.IPv4(10, 100, 101, 1)

	frame := declined(t, addr, server)
	payload := frame[ethHeader+ipHeader+udpHeader:]

	msg, err := dhcpv4.FromBytes(payload)
	if err != nil {
		t.Fatalf("the server could not parse it: %v", err)
	}

	if got := msg.MessageType(); got != dhcpv4.MessageTypeDecline {
		t.Errorf("message type = %v, want a decline", got)
	}
	if got := msg.RequestedIPAddress(); !got.Equal(addr.To4()) {
		t.Errorf("requested address = %v, want %v", got, addr)
	}
	if got := msg.ServerIdentifier(); !got.Equal(server.To4()) {
		t.Errorf("server identifier = %v, want %v", got, server)
	}
	if !bytes.Equal(msg.ClientHWAddr, testMAC) {
		t.Errorf("hardware address = %v, want %v", msg.ClientHWAddr, testMAC)
	}

	// The address goes in the option, not in ciaddr: ciaddr is for one the client is using, and
	// the point of a decline is that it never started.
	if !msg.ClientIPAddr.Equal(net.IPv4zero) {
		t.Errorf("ciaddr = %v, want it empty", msg.ClientIPAddr)
	}
}

// A lease from a server that did not name itself. Inventing an identifier would address the
// decline to nobody, so there is simply no option.
func TestADeclineWithNoServerOmitsTheIdentifier(t *testing.T) {
	frame := declined(t, net.IPv4(10, 100, 101, 106), nil)
	payload := frame[ethHeader+ipHeader+udpHeader:]

	msg, err := dhcpv4.FromBytes(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got := msg.ServerIdentifier(); got != nil {
		t.Errorf("server identifier = %v, want none", got)
	}
	if got := msg.MessageType(); got != dhcpv4.MessageTypeDecline {
		t.Errorf("message type = %v, want a decline", got)
	}
}

func TestDecliningSomethingThatIsNotAnAddress(t *testing.T) {
	if _, err := declineMessage(testMAC, net.ParseIP("fd06::1"), nil, "why"); err == nil {
		t.Error("an IPv6 address was accepted")
	}
}

func TestADeclineNeedsAnEthernetAddress(t *testing.T) {
	if got := declineFrame(net.HardwareAddr{0x00, 0x01}, []byte{1, 2, 3}); got != nil {
		t.Error("a short hardware address produced a frame")
	}
}
