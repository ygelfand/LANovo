package mtkaudio

import (
	"slices"
	"testing"

	"github.com/ygelfand/LANovo/internal/board"
)

func TestStockTables(t *testing.T) {
	for set, want := range map[string]map[string]int{
		"ivy":  {"amp init": 1667, "amp hiz": 28, "amp play": 3, "amp sleep": 3, "amp mute": 6, "mic init": 46},
		"cube": {"amp init": 1666, "amp hiz": 11, "amp play": 3, "amp sleep": 3, "amp mute": 6, "mic init": 46},
	} {
		tb, err := Stock(set)
		if err != nil {
			t.Fatal(err)
		}
		for name, rows := range map[string]Rows{
			"amp init":  tb.AmpInit,
			"amp hiz":   tb.AmpHiZ,
			"amp play":  tb.AmpPlay,
			"amp sleep": tb.AmpSleep,
			"amp mute":  tb.AmpMute,
			"mic init":  tb.MicInit,
		} {
			if len(rows) != want[name] {
				t.Errorf("%s %s: %d rows, want %d", set, name, len(rows), want[name])
			}
		}
	}
	if _, err := Stock(""); err == nil {
		t.Error("an unnamed set loaded")
	}
}

func TestEveryMediaTekChipHasItsTables(t *testing.T) {
	for _, b := range []board.Board{board.Ivy, board.Cube} {
		for _, c := range []*board.Chip{b.Amp, b.MicADC} {
			if c == nil {
				t.Fatalf("%s is missing a chip", b.Name)
			}
			if _, err := Stock(c.Table); err != nil {
				t.Errorf("%s: %v", b.Name, err)
			}
		}
	}
	if !slices.Equal(must(Stock("cube")).AmpPlay, must(Stock("ivy")).AmpPlay) {
		t.Error("the cube's play state differs from ivy's")
	}
}

func must(t Tables, err error) Tables {
	if err != nil {
		panic(err)
	}
	return t
}
