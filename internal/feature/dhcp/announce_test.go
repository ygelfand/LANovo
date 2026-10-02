package dhcp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

var testMAC = net.HardwareAddr{0x00, 0xf4, 0x8d, 0x47, 0x69, 0x21}

// The frame is what every other DHCP client on the network sends when it takes a lease, and what
// an access point doing proxy ARP learns a client from. Checked field by field, because a frame
// that is almost right is one nothing answers and nothing complains about.
func TestArpAnnounceFrame(t *testing.T) {
	ip := net.IPv4(10, 100, 101, 106)
	frame := arpAnnounce(testMAC, ip)

	if len(frame) != ethHeader+arpPayload {
		t.Fatalf("frame is %d bytes, want %d", len(frame), ethHeader+arpPayload)
	}

	if !bytes.Equal(frame[0:6], broadcast) {
		t.Error("the frame is not addressed to the broadcast address")
	}
	if !bytes.Equal(frame[6:12], testMAC) {
		t.Error("the frame does not come from this device")
	}
	if got := binary.BigEndian.Uint16(frame[12:14]); got != 0x0806 {
		t.Errorf("ethertype = %#04x, want 0x0806", got)
	}

	arp := frame[ethHeader:]
	for _, f := range []struct {
		name string
		got  uint16
		want uint16
	}{
		{"hardware type", binary.BigEndian.Uint16(arp[0:2]), 1},
		{"protocol type", binary.BigEndian.Uint16(arp[2:4]), 0x0800},
		{"operation", binary.BigEndian.Uint16(arp[6:8]), 1},
	} {
		if f.got != f.want {
			t.Errorf("%s = %d, want %d", f.name, f.got, f.want)
		}
	}
	if arp[4] != 6 || arp[5] != 4 {
		t.Errorf("address lengths are %d and %d, want 6 and 4", arp[4], arp[5])
	}
}

// Sender and target are both this device: asking about itself is what makes it an announcement
// rather than a question, and it is why nothing answers.
func TestArpAnnounceAsksAboutItself(t *testing.T) {
	ip := net.IPv4(10, 100, 101, 106)
	arp := arpAnnounce(testMAC, ip)[ethHeader:]

	if !bytes.Equal(arp[8:14], testMAC) {
		t.Error("the sender hardware address is not this device")
	}
	if !net.IP(arp[14:18]).Equal(ip) {
		t.Errorf("sender address = %v, want %v", net.IP(arp[14:18]), ip)
	}
	if !net.IP(arp[24:28]).Equal(ip) {
		t.Errorf("target address = %v, want %v", net.IP(arp[24:28]), ip)
	}

	// What an answer would carry, so it is left empty.
	if !bytes.Equal(arp[18:24], make([]byte, 6)) {
		t.Error("the target hardware address is filled in")
	}
}

// The address arrives from a lease in whichever form net chose, and the frame has room for four
// bytes. A sixteen byte spelling copied in whole would overrun the sender field into the target.
func TestArpAnnounceTakesTheSixteenByteForm(t *testing.T) {
	four := arpAnnounce(testMAC, net.IPv4(10, 100, 101, 106).To4())
	sixteen := arpAnnounce(testMAC, net.IPv4(10, 100, 101, 106).To16())

	if !bytes.Equal(four, sixteen) {
		t.Error("the same address written two ways produced different frames")
	}
}

// Nothing to announce is not a frame to send.
func TestArpAnnounceRefusesWhatItCannotSend(t *testing.T) {
	if arpAnnounce(testMAC, net.ParseIP("fd06:16d4:6833:e408::1")) != nil {
		t.Error("an IPv6 address produced a frame")
	}
	if arpAnnounce(net.HardwareAddr{0x00, 0x01}, net.IPv4(10, 0, 0, 1)) != nil {
		t.Error("a short hardware address produced a frame")
	}
}
