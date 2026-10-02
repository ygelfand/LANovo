package dex

import (
	"fmt"
	"reflect"
	"testing"
)

func TestAnArrayLiteralComesBackAsItsRows(t *testing.T) {
	var code []uint16
	var fixups []int
	emit := func(w ...uint16) { code = append(code, w...) }
	fill := func(reg uint16) {
		fixups = append(fixups, len(code))
		emit(0x0026|reg<<8, 0, 0)
	}

	emit(0x2012)         // const/4 v0, 2
	emit(0x0123, 0)      // new-array v1, v0, [[S
	emit(0x0223, 1)      // new-array v2, v0, [S
	fill(2)              // fill-array-data v2
	emit(0x0312)         // const/4 v3, 0
	emit(0x024d, 0x0301) // aput-object v2, v1, v3
	emit(0x0223, 1)      // new-array v2, v0, [S
	fill(2)              // fill-array-data v2
	emit(0x1312)         // const/4 v3, 1
	emit(0x024d, 0x0301) // aput-object v2, v1, v3
	emit(0x0169, 0)      // sput-object v1, AMP
	emit(0x000e)         // return-void

	for i, at := range fixups {
		rel := len(code) - at
		code[at+1], code[at+2] = uint16(rel), uint16(rel>>16)
		emit(0x0300, 2, 2, 0, uint16(0x10*(i+1)), uint16(0xff00+i))
	}

	names := func(m map[uint32]string) func(uint32) (string, error) {
		return func(i uint32) (string, error) {
			if s, ok := m[i]; ok {
				return s, nil
			}
			return "", fmt.Errorf("no %d", i)
		}
	}
	got, err := run(code, names(map[uint32]string{0: "AMP"}), names(map[uint32]string{0: "[[S", 1: "[S"}))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][][]uint16{"AMP": {{0x10, 0xff00}, {0x20, 0xff01}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAnythingElseInTheInitializerIsRefused(t *testing.T) {
	if _, err := run([]uint16{0x0071, 0, 0}, nil, nil); err == nil {
		t.Error("an invoke-static was interpreted")
	}
}
