package visuals

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

func From(x visual.Input, s config.Source) visual.Input {
	switch s {
	case config.SourceMic:
		x.Speaker, x.SpeakerLeft, x.SpeakerRight = x.Mic, x.MicLeft, x.MicRight
		x.Replying = false
	case config.SourceSpeaker:
		x.Mic, x.MicLeft, x.MicRight = x.Speaker, x.SpeakerLeft, x.SpeakerRight
		x.Replying = true
	}
	return x
}
