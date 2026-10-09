package volume

import (
	"path/filepath"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	sharedvolume "github.com/ygelfand/libcountertop/pkg/audio/volume"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func fresh(t *testing.T) *Volume {
	t.Helper()

	config.Use(filepath.Join(t.TempDir(), "state.json"))

	v := Get()
	v.Restore(config.Defaults())
	return v
}

func TestRestoreTakesEveryStream(t *testing.T) {
	v := fresh(t)
	defaults := config.Defaults().Volume

	for _, s := range config.Streams() {
		if got, want := v.Level(s), defaults.Level(s); got != want {
			t.Errorf("%s restored to %d, want %d", s, got, want)
		}
	}
}

func TestStreamsAreIndependent(t *testing.T) {
	v := fresh(t)

	v.Set(config.StreamMedia, 10)
	v.Set(config.StreamAlerts, 90)

	if got := v.Level(config.StreamMedia); got != 10 {
		t.Errorf("media = %d, want 10", got)
	}
	if got := v.Level(config.StreamAlerts); got != 90 {
		t.Errorf("alerts = %d, want 90", got)
	}
}

func TestSetClamps(t *testing.T) {
	v := fresh(t)

	v.Set(config.StreamMedia, 500)
	if got := v.Level(config.StreamMedia); got != 100 {
		t.Errorf("above the top came out %d, want 100", got)
	}

	v.Set(config.StreamMedia, -20)
	if got := v.Level(config.StreamMedia); got != 0 {
		t.Errorf("below the bottom came out %d, want 0", got)
	}
}

func TestAdjustMovesOneStep(t *testing.T) {
	v := fresh(t)
	v.Set(config.StreamMedia, 50)

	v.Adjust(config.StreamMedia, 1)
	if got := v.Level(config.StreamMedia); got != 50+Step {
		t.Errorf("after one press up, %d, want %d", got, 50+Step)
	}

	v.Adjust(config.StreamMedia, -1)
	if got := v.Level(config.StreamMedia); got != 50 {
		t.Errorf("after one press down, %d, want 50", got)
	}
}

func TestAdjustStopsAtTheEnds(t *testing.T) {
	v := fresh(t)

	v.Set(config.StreamMedia, 100)
	for range 10 {
		v.Adjust(config.StreamMedia, 1)
	}
	if got := v.Level(config.StreamMedia); got != 100 {
		t.Errorf("held up at the top, %d, want 100", got)
	}

	v.Adjust(config.StreamMedia, -1)
	if got := v.Level(config.StreamMedia); got != 100-Step {
		t.Errorf("one press down from the top, %d, want %d", got, 100-Step)
	}
}

func TestChangedFires(t *testing.T) {
	v := fresh(t)
	v.Set(config.StreamMedia, 50)

	got := make(chan Change, 4)
	cancel := v.Changed.Listen(func(c Change) { got <- c })
	defer cancel()

	v.Set(config.StreamMedia, 60)

	select {
	case c := <-got:
		if c.Stream != config.StreamMedia || c.Level != 60 {
			t.Errorf("Changed carried %+v, want media at 60", c)
		}
	default:
		t.Fatal("Changed did not fire")
	}
}

func TestUnchangedLevelIsNotAnnounced(t *testing.T) {
	v := fresh(t)
	v.Set(config.StreamMedia, 50)

	got := make(chan Change, 4)
	cancel := v.Changed.Listen(func(c Change) { got <- c })
	defer cancel()

	v.Set(config.StreamMedia, 50)

	select {
	case c := <-got:
		t.Errorf("Changed fired for an unchanged level: %+v", c)
	default:
	}
}

func TestSetPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	config.Use(path)

	v := Get()
	v.Restore(config.Defaults())
	v.Set(config.StreamVoice, 17)

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if got := saved.Get().Volume.Voice; got != 17 {
		t.Errorf("voice came back %d, want 17", got)
	}
}

func TestEveryStreamHasAnEntity(t *testing.T) {
	v := fresh(t)
	for _, stream := range config.Streams() {
		number := v.Number(stream)
		found := false
		for _, entity := range v.Entities() {
			if entity == number {
				found = true
			}
		}
		if number == nil || !found {
			t.Fatalf("%s has no exported volume entity", stream)
		}
	}
}

func TestDuckingIsASeparateSavedPlaybackSetting(t *testing.T) {
	v := fresh(t)
	n, ok := v.duck.Entity("duck").(*esphome.Number)
	if !ok || n.ObjectID != "media_duck_level" {
		t.Fatal("no ducking control")
	}
	n.OnCommand(-20)
	if got := config.Get().Media.DuckDB; got != -20 {
		t.Fatal(got)
	}
}

func TestMainDrivesTheSpeakerAndTheStreamsKeepTheirOwn(t *testing.T) {
	v := fresh(t)

	v.Set(config.StreamMain, 50)
	v.Set(config.StreamVoice, 80)

	if got, want := speaker.Get().Volume(), sharedvolume.Gain(50); got != want {
		t.Errorf("main put the speaker at %v, want %v", got, want)
	}
	if got := v.Level(config.StreamMedia); got != config.Defaults().Volume.Media {
		t.Errorf("media moved to %d", got)
	}
}

func TestMuteLeavesTheSavedLevelAndOtherStreams(t *testing.T) {
	v := fresh(t)
	v.Set(config.StreamMedia, 35)

	v.Mute(config.StreamMedia, true)
	if !v.Muted(config.StreamMedia) || v.Muted(config.StreamVoice) {
		t.Error("mute reached the wrong stream")
	}
	if v.Level(config.StreamMedia) != 35 || config.Get().Volume.Media != 35 {
		t.Errorf(
			"muting moved media to %d, saved %d",
			v.Level(config.StreamMedia),
			config.Get().Volume.Media,
		)
	}

	v.Mute(config.StreamMedia, false)
	if v.Muted(config.StreamMedia) || v.Level(config.StreamMedia) != 35 {
		t.Errorf("unmuted at %d", v.Level(config.StreamMedia))
	}
}

func TestTheButtonsTargetMainWithNothingPicked(t *testing.T) {
	v := fresh(t)
	v.Dismiss()
	if got := v.Target(); got != config.StreamMain {
		t.Errorf("buttons target %s, want main", got)
	}
}
