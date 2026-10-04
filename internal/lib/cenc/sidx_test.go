package cenc

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestTheIndexGivesEachFragmentsRangeAndTime(t *testing.T) {
	body := make([]byte, 0, 64)
	body = append(body, 0, 0, 0, 0)
	body = binary.BigEndian.AppendUint32(body, 1)
	body = binary.BigEndian.AppendUint32(body, 1000)
	body = binary.BigEndian.AppendUint32(body, 500)
	body = binary.BigEndian.AppendUint32(body, 10)
	body = append(body, 0, 0, 0, 2)
	for _, r := range [][2]uint32{{300, 2000}, {400, 3000}} {
		body = binary.BigEndian.AppendUint32(body, r[0])
		body = binary.BigEndian.AppendUint32(body, r[1])
		body = binary.BigEndian.AppendUint32(body, 0x90000000)
	}
	b := binary.BigEndian.AppendUint32(nil, uint32(8+len(body)))
	b = append(append(b, "sidx"...), body...)

	got, err := ParseIndex(b, 1000)
	if err != nil {
		t.Fatal(err)
	}
	end := int64(1000 + len(b))
	want := []Fragment{
		{Offset: end + 10, Size: 300, At: 500 * time.Millisecond, Duration: 2 * time.Second},
		{Offset: end + 310, Size: 400, At: 2500 * time.Millisecond, Duration: 3 * time.Second},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestAnIndexReferenceIsMarked(t *testing.T) {
	body := make([]byte, 0, 64)
	body = append(body, 0, 0, 0, 0)
	body = binary.BigEndian.AppendUint32(body, 1)
	body = binary.BigEndian.AppendUint32(body, 1000)
	body = binary.BigEndian.AppendUint32(body, 0)
	body = binary.BigEndian.AppendUint32(body, 0)
	body = append(body, 0, 0, 0, 2)
	for _, r := range []uint32{0x80000000 | 120, 400} {
		body = binary.BigEndian.AppendUint32(body, r)
		body = binary.BigEndian.AppendUint32(body, 1000)
		body = binary.BigEndian.AppendUint32(body, 0x90000000)
	}
	b := binary.BigEndian.AppendUint32(nil, uint32(8+len(body)))
	b = append(append(b, "sidx"...), body...)

	got, err := ParseIndexAt(b, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Index || got[0].Offset != 5000 || got[0].Size != 120 || got[1].Index || got[1].Offset != 5120 {
		t.Fatalf("got %+v", got)
	}
}
