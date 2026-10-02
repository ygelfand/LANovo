package feedback

import (
	"path/filepath"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

// heard swaps the speaker for a list of what reached it, and points the settings at a file of this
// test's own.
func heard(t *testing.T, chime config.Chime) *[][]Note {
	t.Helper()

	config.Use(filepath.Join(t.TempDir(), "state.json"))
	if err := config.Set().Feedback().Chime(chime); err != nil {
		t.Fatal(err)
	}

	var played [][]Note

	was := play
	play = func(notes []Note) { played = append(played, notes) }
	t.Cleanup(func() { play = was })

	return &played
}

// occasions is every way the device makes a noise about itself. Listed here rather than tested one
// at a time, so an occasion added without going through the gate fails this.
var occasions = map[string]func(){
	"failure":  Failure,
	"canceled": Canceled,
	"volume":   Volume,
	"muted":    Muted,
	"unmuted":  Unmuted,
}

// None is a device that has been told to be quiet, and quiet means all of it. Somebody who turned
// the chimes off wants silence rather than a device that still beeps at them for the other reasons.
func TestNoneSilencesEveryOccasion(t *testing.T) {
	played := heard(t, config.ChimeNone)

	for name, occasion := range occasions {
		occasion()
		if len(*played) != 0 {
			t.Errorf("%s made a sound with the chimes off", name)
			*played = nil
		}
	}
}

func TestEveryOccasionSoundsWhenChimesAreOn(t *testing.T) {
	played := heard(t, config.ChimeChirp)

	for name, occasion := range occasions {
		*played = nil

		occasion()
		if len(*played) != 1 {
			t.Errorf("%s played %d times, want once", name, len(*played))
			continue
		}
		if len((*played)[0]) == 0 {
			t.Errorf("%s played no notes", name)
		}
	}
}

// The acknowledgement is the one occasion whose sound is chosen, so it has to follow the setting
// rather than play whatever it played last.
func TestTheAcknowledgementFollowsTheSetting(t *testing.T) {
	for _, chime := range config.Chimes() {
		if chime == config.ChimeNone {
			continue
		}

		played := heard(t, chime)

		Volume()
		if len(*played) != 1 {
			t.Errorf("%v played %d times, want once", chime, len(*played))
			continue
		}
		if want := speaker.ChimeTone(chime); len((*played)[0]) != len(want) {
			t.Errorf("%v played %d notes, want the %d its tone has", chime, len((*played)[0]), len(want))
		}
	}
}

// The tones that carry meaning in their shape are not the chime, and a setting that changed them
// would be choosing what the device is allowed to say rather than how it says it.
func TestTheMeaningfulTonesIgnoreTheSetting(t *testing.T) {
	fixed := map[string]func(){"failure": Failure, "muted": Muted, "unmuted": Unmuted}

	for name, occasion := range fixed {
		var first []Note

		for _, chime := range config.Chimes() {
			if chime == config.ChimeNone {
				continue
			}

			played := heard(t, chime)

			occasion()
			if len(*played) != 1 {
				t.Fatalf("%s played %d times under %v", name, len(*played), chime)
			}

			if first == nil {
				first = (*played)[0]
				continue
			}
			if len((*played)[0]) != len(first) {
				t.Errorf("%s changed shape under %v", name, chime)
			}
		}
	}
}
