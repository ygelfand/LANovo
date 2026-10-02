package gui

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/feature/shell"
)

type view struct{ covers bool }

func (v *view) Covers() bool { return v.covers }

func TestTheHighestCoveringViewIsTheBaseAndWhatIsAboveItFloats(t *testing.T) {
	under, page, card := &view{covers: true}, &view{covers: true}, &view{}
	n := &Nav{For: func(v shell.View) *Screen { return &Screen{View: v} }}
	covering, above := n.Showing([]shell.View{under, page, card})
	if covering == nil || covering.View != page {
		t.Fatalf("covering %v, want the page", covering)
	}
	if len(above) != 1 || above[0].View != card {
		t.Errorf("above %v, want the card", above)
	}
	if covering, _ := n.Showing([]shell.View{card}); covering != nil {
		t.Errorf("an overlay alone was treated as covering")
	}
}

func TestAViewNobodyDrawsIsLeftOut(t *testing.T) {
	known, unknown := &view{covers: true}, &view{}
	n := &Nav{For: func(v shell.View) *Screen {
		if v == known {
			return &Screen{View: v}
		}
		return nil
	}}
	covering, above := n.Showing([]shell.View{known, unknown})
	if covering == nil || len(above) != 0 {
		t.Errorf("covering %v above %v", covering, above)
	}
}

func TestOnlyAFastSweepFromTheRightEdgeGoesBack(t *testing.T) {
	const w = 1024
	cases := []struct {
		from [2]int
		x, y int
		want bool
	}{
		{[2]int{1000, 300}, 800, 310, true},
		{[2]int{600, 300}, 400, 300, false},
		{[2]int{1000, 300}, 950, 300, false},
		{[2]int{1000, 100}, 850, 400, false},
		{[2]int{960, 150}, 800, 150, false},
	}
	for _, c := range cases {
		if got := backSweep(c.from, c.x, c.y, w); got != c.want {
			t.Errorf("from %v to %d,%d: %v, want %v", c.from, c.x, c.y, got, c.want)
		}
	}
}
