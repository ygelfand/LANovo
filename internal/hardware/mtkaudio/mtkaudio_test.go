package mtkaudio

import "testing"

func TestStockTables(t *testing.T) {
	tb, err := Stock()
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		rows Rows
		want int
	}{
		"amp init":  {tb.AmpInit, 1667},
		"amp hiz":   {tb.AmpHiZ, 28},
		"amp play":  {tb.AmpPlay, 3},
		"amp sleep": {tb.AmpSleep, 3},
		"amp mute":  {tb.AmpMute, 6},
		"mic init":  {tb.MicInit, 46},
	} {
		if len(c.rows) != c.want {
			t.Errorf("%s: %d rows, want %d", name, len(c.rows), c.want)
		}
	}
}
