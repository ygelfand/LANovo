package config

import (
	"slices"
	"testing"
)

func TestHomePicksSurviveARestart(t *testing.T) {
	path := fresh(t)

	want := HomePick{Labels: []string{"lanovo"}, Areas: []string{"kitchen"}, Entities: []string{"light.a"}}
	if err := Set().Home().Pick("lights", want); err != nil {
		t.Fatal(err)
	}
	if err := Set().Home().Pick("switches", HomePick{All: true}); err != nil {
		t.Fatal(err)
	}

	Use(path)
	home := Get().Home
	got := home.Picked("lights")
	if got.All || !slices.Equal(got.Labels, want.Labels) || !slices.Equal(got.Areas, want.Areas) || !slices.Equal(got.Entities, want.Entities) {
		t.Errorf("lights came back as %+v", got)
	}
	if !home.Picked("switches").All {
		t.Errorf("switches came back as %+v", home.Picked("switches"))
	}
}

func TestHomePickDoesNotChangeWhatWasRead(t *testing.T) {
	fresh(t)

	if err := Set().Home().Pick("lights", HomePick{Entities: []string{"light.a"}}); err != nil {
		t.Fatal(err)
	}
	before := Get().Home

	if err := Set().Home().Pick("lights", HomePick{}); err != nil {
		t.Fatal(err)
	}
	if got := before.Picked("lights").Entities; !slices.Equal(got, []string{"light.a"}) {
		t.Errorf("an earlier read changed to %v", got)
	}
	if got := Get().Home.Picked("lights"); !got.Empty() {
		t.Errorf("cleared lights read as %+v", got)
	}
}

func TestAllKeepsTheManualPicks(t *testing.T) {
	fresh(t)

	p := HomePick{Areas: []string{"kitchen"}, Entities: []string{"light.a"}}
	p.All = true
	if err := Set().Home().Pick("lights", p); err != nil {
		t.Fatal(err)
	}
	p.All = false
	if err := Set().Home().Pick("lights", p); err != nil {
		t.Fatal(err)
	}
	got := Get().Home.Picked("lights")
	if got.All || !slices.Equal(got.Areas, []string{"kitchen"}) || !slices.Equal(got.Entities, []string{"light.a"}) {
		t.Errorf("after All on and off: %+v", got)
	}
}
