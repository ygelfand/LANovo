package sdp

import (
	"bytes"
	"errors"
	"testing"
)

func roundTrip(t *testing.T, e Element) Element {
	t.Helper()

	b, err := e.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, n, err := ParseElement(b)
	if err != nil {
		t.Fatalf("ParseElement: %v", err)
	}
	if n != len(b) {
		t.Errorf("parsing took %d of %d bytes", n, len(b))
	}
	return got
}

// A number is written in the smallest width that holds it, and the width is in the tag rather than
// beside it.
func TestANumberTakesTheSmallestWidthThatHoldsIt(t *testing.T) {
	for _, tc := range []struct {
		v     uint32
		bytes int
	}{
		{0, 1},
		{255, 1},
		{256, 2},
		{0xffff, 2},
		{0x10000, 4},
	} {
		b, err := Uint(tc.v).Marshal()
		if err != nil {
			t.Fatalf("Marshal %d: %v", tc.v, err)
		}
		// One tag byte, then the value.
		if got := len(b) - 1; got != tc.bytes {
			t.Errorf("%d took %d bytes, want %d", tc.v, got, tc.bytes)
		}

		got := roundTrip(t, Uint(tc.v))
		if v, ok := got.Uint(); !ok || v != tc.v {
			t.Errorf("%d came back %d (ok=%v)", tc.v, v, ok)
		}
	}
}

// Every assigned Bluetooth identifier is a short UUID, and an audio sink is 0x110b.
func TestAShortUUIDGoesOutAndComesBack(t *testing.T) {
	got := roundTrip(t, UUID16(0x110b))

	if got.Type != TypeUUID {
		t.Errorf("came back as type %d, want a uuid", got.Type)
	}
	if v, ok := got.Uint(); !ok || v != 0x110b {
		t.Errorf("came back %#x", v)
	}
	if !bytes.Equal(got.Value, []byte{0x11, 0x0b}) {
		t.Errorf("the bytes are %x, want big endian 110b", got.Value)
	}
}

func TestTextAndBoolAndNil(t *testing.T) {
	if got := roundTrip(t, Text("LANovo")); string(got.Value) != "LANovo" {
		t.Errorf("text came back %q", got.Value)
	}
	if got := roundTrip(t, Bool(true)); got.Type != TypeBool || got.Value[0] != 1 {
		t.Errorf("true came back %+v", got)
	}
	if got := roundTrip(t, Bool(false)); got.Value[0] != 0 {
		t.Errorf("false came back %+v", got)
	}

	got := roundTrip(t, Nil())
	if got.Type != TypeNil || len(got.Value) != 0 {
		t.Errorf("nil came back %+v", got)
	}
}

// Most of a service record is sequences inside sequences, so nesting has to survive the trip.
func TestSequencesNest(t *testing.T) {
	want := Sequence(
		UUID16(0x0100), // L2CAP
		Uint(0x0019),   // the AVDTP channel
		Sequence(
			UUID16(0x0019),
			Uint(0x0103),
		),
	)

	got := roundTrip(t, want)
	if got.Type != TypeSequence {
		t.Fatalf("came back as type %d", got.Type)
	}
	if len(got.Children) != 3 {
		t.Fatalf("%d children came back, want 3", len(got.Children))
	}

	if v, _ := got.Children[0].Uint(); v != 0x0100 {
		t.Errorf("the first child is %#x", v)
	}

	inner := got.Children[2]
	if inner.Type != TypeSequence || len(inner.Children) != 2 {
		t.Fatalf("the nested sequence came back %+v", inner)
	}
	if v, _ := inner.Children[1].Uint(); v != 0x0103 {
		t.Errorf("the nested version is %#x", v)
	}
}

// The length is written in as few bytes as hold it, and a record long enough to need two is the
// case that catches an encoder that only ever writes one.
func TestALongSequenceUsesAWiderLength(t *testing.T) {
	var children []Element
	for range 100 {
		children = append(children, Uint(0x11223344))
	}

	b, err := Sequence(children...).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// 100 children of five bytes each is 500, which does not fit one length byte.
	if index := b[0] & 0x7; index != size16b {
		t.Errorf("a 500 byte sequence used size index %d, want the two byte length", index)
	}

	got, _, err := ParseElement(b)
	if err != nil {
		t.Fatalf("ParseElement: %v", err)
	}
	if len(got.Children) != 100 {
		t.Errorf("%d children came back, want 100", len(got.Children))
	}
}

// A buffer that has not all arrived is a short read, which a caller tells apart from a malformed
// one.
func TestAnElementThatHasNotAllArrived(t *testing.T) {
	whole, err := Sequence(UUID16(0x110b), Text("a name")).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	for n := range len(whole) {
		if _, _, err := ParseElement(whole[:n]); !errors.Is(err, ErrShort) {
			t.Errorf("%d of %d bytes gave %v, want a short read", n, len(whole), err)
		}
	}
}

func TestWhatIsNotAnElement(t *testing.T) {
	// Type 9 and up are not defined.
	if _, _, err := ParseElement([]byte{9 << 3, 0}); err == nil {
		t.Error("an undefined type was accepted")
	}
	if _, err := (Element{Type: 30}).Marshal(); err == nil {
		t.Error("an undefined type was written")
	}

	// A fixed width type cannot be three bytes.
	if _, err := (Element{Type: TypeUint, Value: []byte{1, 2, 3}}).Marshal(); err == nil {
		t.Error("a three byte integer was written")
	}
}

// A child that runs past the end of its sequence is a malformed record: the length already said
// where the sequence stops, so this is not more to read.
func TestASequenceWhoseChildOverrunsIt(t *testing.T) {
	// A sequence of two bytes holding a uint that claims four.
	bad := []byte{TypeSequence<<3 | size8b, 0x02, TypeUint<<3 | size4, 0xff}

	if _, _, err := ParseElement(bad); err == nil {
		t.Error("a child running past its sequence was accepted")
	}
}

func TestTheTagPacksTypeAndSizeTogether(t *testing.T) {
	b, err := UUID16(0x110b).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if got := b[0] >> 3; got != TypeUUID {
		t.Errorf("the tag says type %d, want a uuid", got)
	}
	if got := b[0] & 0x7; got != size2 {
		t.Errorf("the tag says size index %d, want two bytes", got)
	}
}
