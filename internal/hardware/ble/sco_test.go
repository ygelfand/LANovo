package ble

import (
	"bytes"
	"errors"
	"testing"
)

func scoBytes(handle uint16, data ...byte) []byte {
	return append([]byte{typeSCO, byte(handle), byte(handle >> 8), byte(len(data))}, data...)
}

func TestAnSCOPacketIsFramedAsItself(t *testing.T) {
	buf := scoBytes(0x0006|2<<12, 0x02, 0x04, 0x02, 0x04)
	got, took, err := parsePacket(buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.kind != typeSCO || got.sco.Handle != 6 || got.sco.Status != 2 || !bytes.Equal(got.sco.Data, []byte{2, 4, 2, 4}) || took != len(buf) {
		t.Errorf("read %+v, took %d of %d", got, took, len(buf))
	}
	for n := range len(buf) {
		if _, _, err := parsePacket(buf[:n]); !errors.Is(err, errShort) {
			t.Errorf("cut to %d bytes: %v, want short", n, err)
		}
	}
}

func TestSCOAmongEventsAndDataKeepsTheLineInStep(t *testing.T) {
	var line []byte
	line = append(line, typeEvent, 0x13, 0x01, 0xaa)
	line = append(line, scoBytes(6, 0x04, 0x0e, 0x02, 0x04, 0x02)...)
	line = append(line, typeACL, 0x01, 0x20, 0x01, 0x00, 0x7f)
	line = append(line, scoBytes(6, 0x04, 0x04, 0x04)...)
	line = append(line, typeEvent, 0x0e, 0x01, 0xbb)

	s := &stream{held: line}
	s.drain(nil)
	if len(s.held) != 0 {
		t.Fatalf("%d bytes left unparsed", len(s.held))
	}
	var codes []byte
	for {
		e, ok := s.take()
		if !ok {
			break
		}
		codes = append(codes, e.Code)
	}
	if !bytes.Equal(codes, []byte{0x13, 0x0e}) {
		t.Errorf("events %x, want 13 0e", codes)
	}
	if a, ok := s.takeACL(); !ok || !bytes.Equal(a.Data, []byte{0x7f}) {
		t.Errorf("acl %+v %v", a, ok)
	}
	if s.scos != 2 || len(s.sco) != 2 || !bytes.Equal(s.sco[1].Data, []byte{4, 4, 4}) {
		t.Errorf("sco count %d kept %d", s.scos, len(s.sco))
	}
}

func TestOnlyTheLatestSCOIsKept(t *testing.T) {
	var q pending
	for i := range scoKept + 10 {
		q.add(packet{kind: typeSCO, sco: scoPacket{Data: []byte{byte(i)}}})
	}
	if q.scos != scoKept+10 || len(q.sco) != scoKept || q.sco[0].Data[0] != 10 || q.sco[scoKept-1].Data[0] != scoKept+9 {
		t.Errorf("counted %d, kept %d from %d", q.scos, len(q.sco), q.sco[0].Data[0])
	}
}
