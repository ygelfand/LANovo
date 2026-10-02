package video

import (
	"encoding/binary"
	"testing"
)

func TestAnnexBSplitsAtAccessUnitDelimiters(t *testing.T) {
	aud := []byte{0, 0, 0, 1, 9, 0xf0}
	idr := []byte{0, 0, 0, 1, 0x65, 1, 2, 3}
	stream := append(append(append(append([]byte{}, aud...), idr...), aud...), idr...)
	codec, w, h, units := Split(stream)
	if codec != H264 || w != 1920 || h != 1080 {
		t.Errorf("codec %08x %dx%d", codec, w, h)
	}
	if len(units) != 2 || len(units[0]) != len(aud)+len(idr) {
		t.Errorf("units %d, first %d bytes", len(units), len(units[0]))
	}
}

func TestIVFGivesItsFramesAndSize(t *testing.T) {
	hdr := make([]byte, 32)
	copy(hdr, "DKIF")
	binary.LittleEndian.PutUint16(hdr[6:], 32)
	copy(hdr[8:], "VP90")
	binary.LittleEndian.PutUint16(hdr[12:], 640)
	binary.LittleEndian.PutUint16(hdr[14:], 360)
	frame := func(n int) []byte {
		f := make([]byte, 12+n)
		binary.LittleEndian.PutUint32(f, uint32(n))
		return f
	}
	stream := append(append(hdr, frame(5)...), frame(3)...)
	codec, w, h, units := Split(stream)
	if codec != VP9 || w != 640 || h != 360 {
		t.Errorf("codec %08x %dx%d", codec, w, h)
	}
	if len(units) != 2 || len(units[0]) != 5 || len(units[1]) != 3 {
		t.Errorf("units %v", units)
	}
}
