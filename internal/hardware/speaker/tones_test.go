package speaker

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

// Every chime the settings offer has to be one the speaker knows, or picking it leaves a device that
// silently stopped acknowledging anything.
func TestEveryOfferedChimeHasNotes(t *testing.T) {
	for _, c := range config.Chimes() {
		notes := ChimeTone(c)

		if c == config.ChimeNone {
			if len(notes) != 0 {
				t.Errorf("None sounds %d notes", len(notes))
			}
			continue
		}
		if len(notes) == 0 {
			t.Errorf("%v has no notes", c)
		}
		if Length(notes) == 0 {
			t.Errorf("%v lasts no time", c)
		}
	}
}

// Told apart by shape, so two of them are not the same sound under two names.
func TestTheChimesAreDistinguishable(t *testing.T) {
	seen := map[string]config.Chime{}

	for _, c := range config.Chimes() {
		if c == config.ChimeNone {
			continue
		}

		var key string
		for _, n := range ChimeTone(c) {
			key += string(rune(int(n.Freq))) + string(rune(n.Ms))
		}
		if had, ok := seen[key]; ok {
			t.Errorf("%v sounds exactly like %v", c, had)
		}
		seen[key] = c
	}
}

// A name from a newer build, or a hand-edited state file, reaches here as a chime with no entry.
// Nothing to play is the right answer: the alternative is a map lookup panicking on the audio path.
func TestAnUnknownChimePlaysNothing(t *testing.T) {
	if notes := ChimeTone(config.Chime("harpsichord")); notes != nil {
		t.Errorf("an unknown chime sounds %v", notes)
	}
}
