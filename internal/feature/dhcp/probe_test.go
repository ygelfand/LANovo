package dhcp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

var otherMAC = net.HardwareAddr{0x02, 0x42, 0xac, 0x11, 0x00, 0x02}

// A probe is an announcement with the sender's address left out. That one field is the whole
// difference: filled in, it is a claim, and claiming an address before finding out whether it is
// free is what probing exists to avoid.
func TestArpProbeClaimsNothing(t *testing.T) {
	ip := net.IPv4(10, 100, 101, 106)
	arp := arpProbe(testMAC, ip)[ethHeader:]

	if !bytes.Equal(arp[14:18], make([]byte, 4)) {
		t.Errorf("sender address = %v, want it left empty", net.IP(arp[14:18]))
	}
	if !net.IP(arp[24:28]).Equal(ip) {
		t.Errorf("target address = %v, want %v", net.IP(arp[24:28]), ip)
	}
	if !bytes.Equal(arp[8:14], testMAC) {
		t.Error("the sender hardware address is not this device")
	}
}

func TestArpProbeFrame(t *testing.T) {
	frame := arpProbe(testMAC, net.IPv4(10, 100, 101, 106))

	if len(frame) != ethHeader+arpPayload {
		t.Fatalf("frame is %d bytes, want %d", len(frame), ethHeader+arpPayload)
	}
	if !bytes.Equal(frame[0:6], broadcast) {
		t.Error("the probe is not broadcast, so the host holding the address may not hear it")
	}
	if got := binary.BigEndian.Uint16(frame[12:14]); got != 0x0806 {
		t.Errorf("ethertype = %#04x, want 0x0806", got)
	}
	if got := binary.BigEndian.Uint16(frame[ethHeader+6 : ethHeader+8]); got != 1 {
		t.Errorf("operation = %d, want a request", got)
	}
}

func TestArpProbeRefusesWhatItCannotSend(t *testing.T) {
	if arpProbe(testMAC, net.ParseIP("fd06:16d4:6833:e408::1")) != nil {
		t.Error("an IPv6 address produced a frame")
	}
	if arpProbe(net.HardwareAddr{0x00, 0x01}, net.IPv4(10, 0, 0, 1)) != nil {
		t.Error("a short hardware address produced a frame")
	}
}

// arp builds a frame to feed the reader, so each case can say what it is rather than be a slice of
// bytes.
func arp(op uint16, senderMAC net.HardwareAddr, senderIP, targetIP net.IP) []byte {
	frame := make([]byte, ethHeader+arpPayload)

	copy(frame[0:6], broadcast)
	copy(frame[6:12], senderMAC)
	binary.BigEndian.PutUint16(frame[12:14], 0x0806)

	a := frame[ethHeader:]
	binary.BigEndian.PutUint16(a[0:2], 1)
	binary.BigEndian.PutUint16(a[2:4], 0x0800)
	a[4], a[5] = 6, 4
	binary.BigEndian.PutUint16(a[6:8], op)

	copy(a[8:14], senderMAC)
	if ip := senderIP.To4(); ip != nil {
		copy(a[14:18], ip)
	}
	if ip := targetIP.To4(); ip != nil {
		copy(a[24:28], ip)
	}
	return frame
}

func TestWhatCountsAsTheAddressBeingTaken(t *testing.T) {
	want := net.IPv4(10, 100, 101, 106)
	other := net.IPv4(10, 100, 101, 50)

	tests := []struct {
		name  string
		frame []byte
		taken bool
	}{
		{
			"somebody answers the probe",
			arp(2, otherMAC, want, net.IPv4zero),
			true,
		},
		{
			"somebody is using it and asks about something else",
			arp(1, otherMAC, want, other),
			true,
		},
		{
			"somebody else is probing for the same address",
			arp(1, otherMAC, net.IPv4zero, want),
			true,
		},
		{
			"our own probe, heard back on the socket that sent it",
			arp(1, testMAC, net.IPv4zero, want),
			false,
		},
		{
			"our own announcement",
			arp(1, testMAC, want, want),
			false,
		},
		{
			"an unrelated host going about its business",
			arp(1, otherMAC, other, net.IPv4(10, 100, 101, 1)),
			false,
		},
		{
			"somebody probing for a different address",
			arp(1, otherMAC, net.IPv4zero, other),
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := taken(tt.frame, testMAC, want); got != tt.taken {
				t.Errorf("taken = %v, want %v", got, tt.taken)
			}
		})
	}
}

// The socket is bound to ARP, but a reader that trusts that and indexes anyway is one bad frame
// from a panic. None of these are evidence of anything either.
func TestMalformedFramesAreNotEvidence(t *testing.T) {
	want := net.IPv4(10, 100, 101, 106)
	good := arp(2, otherMAC, want, net.IPv4zero)

	short := make([]byte, len(good))
	copy(short, good)

	notARP := make([]byte, len(good))
	copy(notARP, good)
	binary.BigEndian.PutUint16(notARP[12:14], 0x0800)

	wrongHardware := make([]byte, len(good))
	copy(wrongHardware, good)
	binary.BigEndian.PutUint16(wrongHardware[ethHeader:ethHeader+2], 6)

	wrongLengths := make([]byte, len(good))
	copy(wrongLengths, good)
	wrongLengths[ethHeader+4] = 8

	reserved := make([]byte, len(good))
	copy(reserved, good)
	binary.BigEndian.PutUint16(reserved[ethHeader+6:ethHeader+8], 9)

	tests := []struct {
		name  string
		frame []byte
	}{
		{"nothing at all", nil},
		{"a runt", short[:20]},
		{"not ARP", notARP},
		{"a hardware type that is not ethernet", wrongHardware},
		{"address lengths that are not 6 and 4", wrongLengths},
		{"an operation that is neither a request nor a reply", reserved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if taken(tt.frame, testMAC, want) {
				t.Error("read as the address being taken")
			}
		})
	}
}

// The address arrives from a lease in whichever form net chose, and a sixteen byte spelling has
// to compare the same as a four byte one.
func TestTakenAcceptsEitherSpellingOfTheAddress(t *testing.T) {
	want := net.IPv4(10, 100, 101, 106)
	frame := arp(2, otherMAC, want, net.IPv4zero)

	if !taken(frame, testMAC, want.To4()) {
		t.Error("the four byte form was not recognized")
	}
	if !taken(frame, testMAC, want.To16()) {
		t.Error("the sixteen byte form was not recognized")
	}
	if taken(frame, testMAC, net.ParseIP("fd06:16d4:6833:e408::1")) {
		t.Error("an IPv6 address matched an ARP frame")
	}
}
