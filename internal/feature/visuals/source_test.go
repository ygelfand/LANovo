package visuals

import (
	"testing"

	"github.com/ygelfand/libcountertop/pkg/audio/analysis"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

func heard() visual.Input {
	return visual.Input{
		Mic: analysis.Analysis{
			Level: 0.1,
		},
		MicLeft:  analysis.Analysis{Level: 0.11},
		MicRight: analysis.Analysis{Level: 0.12},
		Speaker: analysis.Analysis{
			Level: 0.5,
		},
		SpeakerLeft:  analysis.Analysis{Level: 0.51},
		SpeakerRight: analysis.Analysis{Level: 0.52},
		Replying:     true,
	}
}

func TestMicOnlyHearsOnlyTheMic(t *testing.T) {
	x := From(heard(), config.SourceMic)
	if x.Speaker.Level != 0.1 || x.SpeakerLeft.Level != 0.11 || x.SpeakerRight.Level != 0.12 ||
		x.Replying {
		t.Errorf("mic only: %+v", x)
	}
	if x.Voice().Level != 0.1 {
		t.Errorf("voice %v", x.Voice().Level)
	}
}

func TestSpeakerOnlyHearsOnlyTheSpeaker(t *testing.T) {
	in := heard()
	in.Replying = false
	x := From(in, config.SourceSpeaker)
	if x.Mic.Level != 0.5 || x.MicLeft.Level != 0.51 || x.MicRight.Level != 0.52 || !x.Replying {
		t.Errorf("speaker only: %+v", x)
	}
	if x.Voice().Level != 0.5 {
		t.Errorf("voice %v", x.Voice().Level)
	}
}

func TestBothLeavesTheInputAlone(t *testing.T) {
	if x := From(heard(), config.SourceBoth); x != heard() {
		t.Errorf("both changed %+v", x)
	}
}
