package settings

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

func TestEveryFrameCapSurvivesTheSlider(t *testing.T) {
	for _, fps := range config.MaxFPSSteps {
		if got := config.MaxFPSSteps[fpsIndex(fpsSnap(fpsLevel(fps)))]; got != fps {
			t.Errorf("%d came back as %d", fps, got)
		}
	}
	if fpsIndex(0) != 0 || fpsIndex(100) != len(config.MaxFPSSteps)-1 {
		t.Error("the ends of the track are not 10 and 60")
	}
}

func TestTheSeedSliderEnds(t *testing.T) {
	if seedOf(0) != 0 || seedOf(100) != visual.SeedMost || seedOf(1) < 1 {
		t.Errorf("ends %d %d %d", seedOf(0), seedOf(1), seedOf(100))
	}
	for _, s := range []int{0, 1, 512, visual.SeedMost} {
		if l := seedLevel(s); (s == 0) != (l == 0) {
			t.Errorf("seed %d sits at %d", s, l)
		}
	}
}
