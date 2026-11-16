package qcomaudio

import (
	"slices"
	"testing"

	"github.com/ygelfand/LANovo/internal/board"
)

func TestEveryQualcommBoardHasItsTable(t *testing.T) {
	for b, want := range map[*board.Board]int{&board.Blueberry: 1535, &board.Amber: 1535, &board.M10: 1159} {
		rows, err := Table(b.Amp.Table)
		if err != nil {
			t.Fatalf("%s: %v", b.Name, err)
		}
		if len(rows) != want {
			t.Errorf("%s: %d rows, want %d", b.Name, len(rows), want)
		}
		if rows[0] != [2]byte{0x00, 0x00} || rows[1] != [2]byte{0x7f, 0x00} {
			t.Errorf("%s: starts %v %v, want page 0 then book 0", b.Name, rows[0], rows[1])
		}
	}
}

func TestTheTablesDifferOnlyInTuning(t *testing.T) {
	blue, err := Table(board.Blueberry.Amp.Table)
	if err != nil {
		t.Fatal(err)
	}
	amber, err := Table(board.Amber.Amp.Table)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 160 {
		if blue[i] != amber[i] {
			t.Fatalf("row %d differs in the setup: %v and %v", i, blue[i], amber[i])
		}
	}
	if slices.Equal(blue, amber) {
		t.Error("amber's table is blueberry's")
	}
}
