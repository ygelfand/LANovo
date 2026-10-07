package idle

import (
	"slices"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

var noon = time.Date(2026, 9, 27, 12, 34, 0, 0, time.UTC)

func TestTwoVisualsSplitTheScreenTheLongWay(t *testing.T) {
	if got := Areas(1920, 1200, 1); !slices.Equal(got, []ui.Rect{{W: 1920, H: 1200}}) {
		t.Errorf("one: %v", got)
	}
	if got := Areas(1920, 1200, 2); !slices.Equal(got, []ui.Rect{{W: 960, H: 1200}, {X: 960, W: 960, H: 1200}}) {
		t.Errorf("landscape: %v", got)
	}
	if got := Areas(1200, 1920, 2); !slices.Equal(got, []ui.Rect{{W: 1200, H: 960}, {Y: 960, W: 1200, H: 960}}) {
		t.Errorf("portrait: %v", got)
	}
	if Areas(1200, 1920, 0) != nil {
		t.Error("areas for no visual")
	}
}

func TestItIsDueOnlyAfterTheWait(t *testing.T) {
	if due(0, time.Hour) {
		t.Error("never came up")
	}
	if due(time.Minute, 30*time.Second) {
		t.Error("came up before the wait")
	}
	if !due(time.Minute, 61*time.Second) {
		t.Error("did not come up after the wait")
	}
}

func idled(change func(*config.Idle)) config.Config {
	cfg := config.Defaults()
	cfg.Clock.Date = false
	change(&cfg.Idle)
	return cfg
}

func TestTheClockTakesItsColoursFromTheVisualUnderIt(t *testing.T) {
	var light theme.Theme
	for _, th := range theme.All {
		if !th.Dark {
			light = th
			break
		}
	}
	dark := []visual.Traits{{}, {}}
	areas := Areas(1920, 1200, 2)

	one := Pieces(light, dark, areas, ui.Rect{X: 100, Y: 100, W: 400, H: 200})
	if len(one) != 1 || !one[0].Palette.Dark || theme.Contrast(one[0].Palette.Text, one[0].Palette.Background) < 4.5 {
		t.Errorf("a light theme over a dark visual gave %+v", one)
	}
	if got := Pieces(light, dark, areas, ui.Rect{X: 700, Y: 100, W: 600, H: 200}); len(got) != 1 || got[0].Clip.W != 0 {
		t.Errorf("a clock across two dark visuals was split: %+v", got)
	}
	if got := Pieces(light, nil, nil, ui.Rect{W: 400, H: 200}); len(got) != 1 || got[0].Palette != light {
		t.Error("with no visual the clock left the theme")
	}
}

func TestAClockAcrossDifferentVisualsIsSplitAtTheSeam(t *testing.T) {
	palette := theme.Default()
	areas := Areas(1920, 1200, 2)
	box := ui.Rect{X: 700, Y: 400, W: 600, H: 300}

	got := Pieces(palette, []visual.Traits{{}, {Light: true}}, areas, box)
	if len(got) != 2 {
		t.Fatalf("%d pieces", len(got))
	}
	if got[0].Clip != (ui.Rect{X: 700, Y: 400, W: 260, H: 300}) || got[1].Clip != (ui.Rect{X: 960, Y: 400, W: 340, H: 300}) {
		t.Errorf("clips %v %v", got[0].Clip, got[1].Clip)
	}
	if !got[0].Palette.Dark || got[1].Palette.Dark {
		t.Error("each half did not take its own visual's tone")
	}
}

func TestOnlyChosenVisualsAreDrawn(t *testing.T) {
	got := chosen(config.Idle{Second: config.IdleVisual{Kind: "orb"}})
	if len(got) != 1 || got[0].Kind != "orb" {
		t.Errorf("chosen %v", got)
	}
}

func TestNoneIsOfferedFirst(t *testing.T) {
	labels := KindLabels()
	if labels[0] != KindLabel("") || len(labels) != len(visual.Built())+1 {
		t.Errorf("labels %v", labels)
	}
	if k, ok := kindByLabel(KindLabel("orb")); !ok || k != "orb" {
		t.Errorf("orb by label: %q %v", k, ok)
	}
}
