package ble

import (
	"bytes"
	"testing"
)

// report builds one advertising report as the controller lays it out: event type, address type,
// address least significant octet first, data length, data, RSSI.
func report(kind, addressType byte, address [6]byte, data []byte, rssi int8) []byte {
	out := []byte{kind, addressType}
	for i := len(address) - 1; i >= 0; i-- {
		out = append(out, address[i])
	}
	out = append(out, byte(len(data)))
	out = append(out, data...)
	return append(out, byte(rssi))
}

// meta wraps reports the way an LE Meta event carries them.
func meta(reports ...[]byte) []byte {
	out := []byte{leAdvertisingReport, byte(len(reports))}
	for _, r := range reports {
		out = append(out, r...)
	}
	return out
}

func TestReportsReadsAnAdvertisement(t *testing.T) {
	want := [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	data := []byte{0x02, 0x01, 0x06}

	var got []Advertisement
	reports(meta(report(0x00, 0x01, want, data, -64)), func(a Advertisement) {
		got = append(got, a)
	})

	if len(got) != 1 {
		t.Fatalf("%d advertisements, want 1", len(got))
	}
	if got[0].Address != want {
		t.Errorf("address %x, want %x", got[0].Address, want)
	}
	if got[0].AddressType != 0x01 {
		t.Errorf("address type %#02x, want 0x01", got[0].AddressType)
	}
	if got[0].RSSI != -64 {
		t.Errorf("rssi %d, want -64", got[0].RSSI)
	}
	if !bytes.Equal(got[0].Data, data) {
		t.Errorf("data % x, want % x", got[0].Data, data)
	}
}

// The wire carries an address backwards and Home Assistant wants it forwards. Reading it the wrong
// way round gives a valid-looking address for a different device, which nothing downstream can
// catch — so the reversal is asserted against a deliberately asymmetric address.
func TestReportsPutsTheAddressTheRightWayRound(t *testing.T) {
	want := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	var got Advertisement
	reports(meta(report(0x00, 0x00, want, nil, -30)), func(a Advertisement) { got = a })

	if got.Address != want {
		t.Fatalf("address %x, want %x", got.Address, want)
	}
	if v := got.Addr(); v != 0x010203040506 {
		t.Errorf("Addr is %#x, want 0x010203040506", v)
	}
}

func TestReportsReadsSeveralInOneEvent(t *testing.T) {
	first := [6]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11}
	second := [6]byte{0x22, 0x22, 0x22, 0x22, 0x22, 0x22}

	var got []Advertisement
	reports(meta(
		report(0x00, 0x00, first, []byte{0x01}, -50),
		report(0x00, 0x00, second, []byte{0x02, 0x03}, -70),
	), func(a Advertisement) { got = append(got, a) })

	if len(got) != 2 {
		t.Fatalf("%d advertisements, want 2", len(got))
	}
	if got[0].Address != first || got[1].Address != second {
		t.Errorf("addresses %x and %x, want %x and %x",
			got[0].Address, got[1].Address, first, second)
	}
	if got[1].RSSI != -70 {
		t.Errorf("the second rssi is %d, want -70", got[1].RSSI)
	}
}

// Anything within radio range can send these, so a malformed one has to be dropped rather than
// read past the end of. Every truncation of a good event is tried, since which byte is missing is
// not something the sender is obliged to be reasonable about.
func TestReportsDropsWhatItCannotRead(t *testing.T) {
	whole := meta(report(0x00, 0x01, [6]byte{1, 2, 3, 4, 5, 6}, []byte{0x02, 0x01, 0x06}, -64))

	for cut := range len(whole) {
		short := whole[:cut]

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%d bytes of a %d byte event panicked: %v", cut, len(whole), r)
				}
			}()
			reports(short, func(Advertisement) {})
		}()
	}
}

// A count that does not match what follows is the same problem from the other end: the event says
// four reports and carries one.
func TestReportsBelievesTheBytesRatherThanTheCount(t *testing.T) {
	said := meta(report(0x00, 0x00, [6]byte{1, 2, 3, 4, 5, 6}, []byte{0x01}, -40))
	said[1] = 4

	seen := 0
	reports(said, func(Advertisement) { seen++ })

	if seen != 1 {
		t.Errorf("%d advertisements read from one report claiming to be four, want 1", seen)
	}
}

// Anything that is not an advertising report belongs to somebody else. Connection complete is the
// one most likely to turn up, since it shares the LE Meta event.
func TestReportsIgnoresOtherLEEvents(t *testing.T) {
	seen := 0
	reports([]byte{0x01, 0x00, 0x00, 0x40, 0x00}, func(Advertisement) { seen++ })

	if seen != 0 {
		t.Errorf("%d advertisements read from a connection complete, want 0", seen)
	}
}
