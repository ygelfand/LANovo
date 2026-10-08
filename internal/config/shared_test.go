package config

import (
	"path/filepath"
	"testing"
)

func TestAWriteLeavesWhatWasReadAlone(t *testing.T) {
	st, err := Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	if err := st.Set().Camera().Set("saturation", "1"); err != nil {
		t.Fatal(err)
	}
	if err := st.Set().Wake(0).MaxThink(3); err != nil {
		t.Fatal(err)
	}

	read := st.Get()

	if err := st.Set().Camera().Set("saturation", "2"); err != nil {
		t.Fatal(err)
	}
	if err := st.Set().Camera().Set("gamma", "3"); err != nil {
		t.Fatal(err)
	}
	if err := st.Set().Wake(0).MaxThink(9); err != nil {
		t.Fatal(err)
	}

	if got := read.Camera.Settings["saturation"]; got != "1" {
		t.Errorf("a write reached into what was read: saturation is %q", got)
	}
	if _, ok := read.Camera.Settings["gamma"]; ok {
		t.Error("a write added a key to what was read")
	}
	if got := read.Wake.Words[0].MaxThink; got != 3 {
		t.Errorf("a write reached into what was read: max think is %d", got)
	}
}
