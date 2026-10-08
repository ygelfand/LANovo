package voice

import (
	"context"

	"github.com/ygelfand/libcountertop/pkg/assistant/turn"
	"github.com/ygelfand/libcountertop/pkg/audio/sound"
	sharedwake "github.com/ygelfand/libcountertop/pkg/inference/wake"
	"github.com/ygelfand/libcountertop/pkg/inference/wakeslots"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/activity"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/recording"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/wake"
)

type voiceMic struct{ *mic.Mics }

func (m voiceMic) Gain() float64 { return m.Leveler.Gain() }

type voiceSpeaker struct{ *speaker.Speaker }

func (s voiceSpeaker) Splices() uint64   { _, n, _, _ := s.Stats(); return n }
func (s voiceSpeaker) Underruns() uint64 { _, _, n, _ := s.Stats(); return n }

type voiceSound struct{ *speaker.Driver }

func (s voiceSound) Claim(name string, fn func(context.Context, turn.Speaker) error) turn.Claim {
	return s.Driver.Claim(
		name,
		func(ctx context.Context, p *speaker.Speaker) error { return fn(ctx, voiceSpeaker{p}) },
	)
}
func (s voiceSound) Duck(on bool) { s.Backgrounds().Duck(on) }

func dependencies() turn.Dependencies {
	return turn.Dependencies{
		Source: voiceMic{
			mic.Get(),
		},
		Speaker:      voiceSpeaker{speaker.Get()},
		Sound:        voiceSound{speaker.Sound()},
		Player:       media.Get(),
		Log:          activity.Get(),
		Recorder:     recording.Get(),
		Models:       func() []sharedwake.Model { return wake.Lib().Ours() },
		HardwareTail: sound.HardwareTail,
		Failure:      feedback.Failure,
		Cancelled:    feedback.Canceled,
		Words:        turn.Words{Settings: config.WakeSection, Speaker: speaker.Sound()},
		Muted:        func() (bool, error) { return privacy.Get().MicMuted(), nil },
		Presentation: &turn.Presentation{Settings: config.WakeSection, Shown: &Shown},
	}
}

func wanted(models []sharedwake.Model, count int) []string {
	saved := config.Get().Wake.Words
	var ids []string
	if saved != nil {
		ids = make([]string, len(saved))
		for i, word := range saved {
			ids[i] = word.ID
		}
	}
	installed := make([]string, len(models))
	for i, m := range models {
		installed[i] = m.ID
	}
	return wakeslots.Initial(ids, installed, sharedwake.DefaultModel, count)
}
