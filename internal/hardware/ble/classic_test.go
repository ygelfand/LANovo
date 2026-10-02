package ble

import (
	"bytes"
	"testing"
	"unicode/utf8"
)

// The name a phone shows beside the device, as the command carries it.

func TestANameIsPaddedToTheFullWidth(t *testing.T) {
	got := localName("Kitchen")

	if len(got) != nameBytes {
		t.Fatalf("%d bytes, want %d", len(got), nameBytes)
	}
	if string(bytes.TrimRight(got, "\x00")) != "Kitchen" {
		t.Errorf("came back %q", bytes.TrimRight(got, "\x00"))
	}
}

func TestAnEmptyNameIsAllPadding(t *testing.T) {
	got := localName("")

	if len(got) != nameBytes {
		t.Fatalf("%d bytes, want %d", len(got), nameBytes)
	}
	if n := len(bytes.TrimRight(got, "\x00")); n != 0 {
		t.Errorf("%d bytes of name in an empty one", n)
	}
}

// A name longer than the command holds is cut at a rune. Cutting at the byte would leave a partial
// one, which a phone draws as a replacement character in the name of the thing it is pairing with.
func TestALongNameIsCutWithoutSplittingARune(t *testing.T) {
	for _, name := range []string{
		"a very long name " + string(bytes.Repeat([]byte("x"), 300)),
		string(bytes.Repeat([]byte("ü"), 200)),
		string(bytes.Repeat([]byte("日"), 200)),
		string(bytes.Repeat([]byte("🎵"), 100)),
	} {
		got := localName(name)

		if len(got) != nameBytes {
			t.Fatalf("%d bytes, want %d", len(got), nameBytes)
		}

		said := string(bytes.TrimRight(got, "\x00"))
		if !utf8.ValidString(said) {
			t.Errorf("cutting %d bytes gave something that is not text", len(name))
		}
		if len(said) > nameBytes {
			t.Errorf("%d bytes of name in a %d byte field", len(said), nameBytes)
		}
	}
}

// The class of device decides the icon a phone draws and whether it offers to send audio at all, so
// it is worth pinning rather than trusting a comment.
func TestTheDeviceSaysItIsALoudspeaker(t *testing.T) {
	// Little endian on the wire: minor, major, service.
	if SpeakerClass != [3]byte{0x14, 0x04, 0x24} {
		t.Fatalf("class of device % x", SpeakerClass)
	}

	major := SpeakerClass[1] & 0x1f
	minor := (SpeakerClass[0] >> 2) & 0x3f

	if major != 0x04 {
		t.Errorf("major class %#02x, want audio/video", major)
	}
	if minor != 0x05 {
		t.Errorf("minor class %#02x, want loudspeaker", minor)
	}
	if SpeakerClass[2]&0x20 == 0 {
		t.Error("the audio service bit is not set")
	}
	if SpeakerClass[2]&0x04 == 0 {
		t.Error("the rendering service bit is not set")
	}
}

// Both scans, or the device is either invisible or unreachable.
func TestDiscoveryIsInquiryAndPage(t *testing.T) {
	if scanDiscovery != 0x03 {
		t.Errorf("scan enable %#02x, want both", scanDiscovery)
	}
}
