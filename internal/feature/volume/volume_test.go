package volume

import (
	"path/filepath"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

// The component is a singleton with a config store behind it, so each test points the store at a
// file of its own and works through Get.
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

// The split is the point: the streams must not share a level.
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

// Holding a volume button at either end must not run the level off the scale, or coming back
// takes as many presses as were wasted.
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

// A command that does not move the level should not be announced, or every repeat of an unchanged
// value wakes whatever is listening.
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

// Every stream needs an entity, or one of them cannot be set from Home Assistant at all.
func TestEveryStreamHasAnEntity(t *testing.T) {
	v := fresh(t)

	if got, want := len(v.Entities()), len(config.Streams()); got != want {
		t.Fatalf("%d entities for %d streams", got, want)
	}
	for _, e := range v.Entities() {
		if e == nil {
			t.Fatal("a stream has no entity")
		}
	}
}

// The panel's proportions, smaller. The card is laid out from fractions of the shorter side and of
// the height, so a canvas of a different shape is a different card.
const (
	testW = 300
	testH = 480
)

// drawn is the card showing one stream, folded or with the rest under it.
