package qcomaudio

import "testing"

func TestStockTable(t *testing.T) {
	rows, err := Stock()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1535 {
		t.Errorf("%d rows, want 1535", len(rows))
	}
	if rows[0] != [2]byte{0x00, 0x00} || rows[1] != [2]byte{0x7f, 0x00} {
		t.Errorf("starts %v %v, want page 0 then book 0", rows[0], rows[1])
	}
}
