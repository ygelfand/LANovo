package feedback

import (
	"path/filepath"
	"testing"

	sharedtone "github.com/ygelfand/libcountertop/pkg/audio/tone"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/config"
)

func heard(t *testing.T, chime schema.Chime) *[][]Note {
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

var occasions = map[string]func(){
	"failure":  Failure,
	"canceled": Canceled,
	"volume":   Volume,
	"muted":    Muted,
	"unmuted":  Unmuted,
}

func TestNoneSilencesEveryOccasion(t *testing.T) {
	played := heard(t, schema.ChimeNone)

	for name, occasion := range occasions {
		occasion()
		if len(*played) != 0 {
			t.Errorf("%s made a sound with the chimes off", name)
			*played = nil
		}
	}
}

func TestEveryOccasionSoundsWhenChimesAreOn(t *testing.T) {
	played := heard(t, schema.ChimeChirp)

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

func TestTheAcknowledgementFollowsTheSetting(t *testing.T) {
	for _, chime := range schema.Chimes() {
		if chime == schema.ChimeNone {
			continue
		}

		played := heard(t, chime)

		Volume()
		if len(*played) != 1 {
			t.Errorf("%v played %d times, want once", chime, len(*played))
			continue
		}
		if want := sharedtone.Wake(chime); len((*played)[0]) != len(want) {
			t.Errorf(
				"%v played %d notes, want the %d its tone has",
				chime,
				len((*played)[0]),
				len(want),
			)
		}
	}
}

func TestTheMeaningfulTonesIgnoreTheSetting(t *testing.T) {
	fixed := map[string]func(){"failure": Failure, "muted": Muted, "unmuted": Unmuted}

	for name, occasion := range fixed {
		var first []Note

		for _, chime := range schema.Chimes() {
			if chime == schema.ChimeNone {
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
