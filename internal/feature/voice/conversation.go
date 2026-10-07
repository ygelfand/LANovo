package voice

import (
	"context"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/activity"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/recording"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/wake"
	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/libcountertop/pkg/assistant/turn"
	"github.com/ygelfand/libcountertop/pkg/inference/wakeslots"
)

type conversation = turn.Conversation
type phase = turn.Phase

const (
	phaseIdle      = turn.Idle
	phaseListening = turn.Listening
	phaseThinking  = turn.Thinking
	phaseReplying  = turn.Replying
)

type voiceMic struct{ *mic.Mics }

func (m voiceMic) Gain() float64 { return m.Leveled() }

type voiceSpeaker struct{ *speaker.Speaker }

func (s voiceSpeaker) Splices() uint64   { _, n, _, _ := s.Stats(); return n }
func (s voiceSpeaker) Underruns() uint64 { _, _, n, _ := s.Stats(); return n }

type voiceSound struct{ *speaker.Driver }

func (s voiceSound) Claim(name string, fn func(context.Context, turn.Speaker) error) turn.Claim {
	return s.Driver.Claim(name, func(ctx context.Context, p *speaker.Speaker) error { return fn(ctx, voiceSpeaker{p}) })
}
func newConversation(vs *esphome.VoiceSatellite) *conversation {
	return turn.NewConversation(vs, turn.Options{
		Source:  voiceMic{mic.Get()},
		Speaker: voiceSpeaker{speaker.Get()},
		Sound:   voiceSound{speaker.Sound()}, Player: media.Get(), Log: activity.Get(), Recorder: recording.Get(),
		Models:   func() []wake.Model { return wake.Lib().Ours() },
		Adapting: mic.Get().SetAdapting, Duck: speaker.Sound().Backgrounds().Duck,
		HardwareTail: speaker.HardwareTail, Shown: Shown.Emit,
		Failure: feedback.Failure,
		Words: turn.Words{
			MaxListen: wakeword.MaxListen, MaxThink: wakeword.MaxThink, FollowUp: wakeword.FollowUp,
			Buffer: wakeword.Buffer, ChimeLength: wakeword.ChimeLength, Tones: wakeword.Tones, Chime: wakeword.Chime,
			Delivery: func(slot int) string { return string(wakeword.Delivery(slot)) },
		},
		Muted: func() (bool, error) { return privacy.Get().MicMuted(), nil }, Cancelled: feedback.Canceled,
	})
}

// wanted retains assistant positions and explicit disables. Only a fresh configuration gets a default.
func wanted(models []wake.Model, count int) []string {
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
	return wakeslots.Initial(ids, installed, wake.DefaultModel, count)
}
