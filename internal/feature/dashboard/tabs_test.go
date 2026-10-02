package dashboard

import "testing"

func TestTabs(t *testing.T) {
	draws := 0
	var offered []Tab
	strip := &tabs{redraw: func() { draws++ }}
	strip.sources = []func() []Tab{
		func() []Tab { return offered },
		func() []Tab { return []Tab{{Kind: "b", Key: "two"}} },
	}

	offered = []Tab{{Kind: "a", Key: "one"}}
	if got := strip.list(); len(got) != 2 || got[0].Key != "one" || got[1].Key != "two" {
		t.Fatalf("tabs %v, want one then two", got)
	}

	strip.show("one")
	if tab, ok := strip.open(); !ok || tab.Kind != "a" {
		t.Errorf("showing %v %v", tab, ok)
	}
	strip.show("one")
	if _, ok := strip.open(); ok {
		t.Error("tapping the open tab again should go back to the clock")
	}

	strip.show("one")
	offered = nil
	if _, ok := strip.open(); ok {
		t.Error("a tab that is no longer offered is still showing")
	}

	strip.show("two")
	strip.clock()
	if _, ok := strip.open(); ok {
		t.Error("Clock left a tab open")
	}
	before := draws
	strip.clock()
	if draws != before {
		t.Error("Clock redrew with nothing open")
	}
}
