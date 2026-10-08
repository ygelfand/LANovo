package voice

import (
	"context"
	"log/slog"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/inference/wakeslots"

	esphome "github.com/ygelfand/go-esphome-device"
	"google.golang.org/protobuf/proto"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/timer"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/wake"
)

func init() {
	component.Register(component.Device, Get, component.Order(30))
}

// Announce and StartConversation both require a media player.
const Features = esphome.DefaultVoiceFeatures |
	esphome.FeatureSpeaker |
	esphome.FeatureAnnounce |
	esphome.FeatureStartConversation |
	esphome.FeatureTimers

type Voice struct {
	vs   *esphome.VoiceSatellite
	turn *conversation

	selection wakeslots.Selection
}

var (
	once   sync.Once
	shared *Voice
)

func Get() *Voice {
	once.Do(func() { shared = build() })
	return shared
}

func build() *Voice {
	ours := wake.Lib().Ours()
	slots := wanted(ours, wakeword.Slots)

	v := &Voice{
		vs: &esphome.VoiceSatellite{
			ActiveWakeWords:     chosen(slots),
			MaxActiveWakeWords:  wakeword.Slots,
			OnExternalWakeWords: wakeword.Answer,
		},
	}
	v.selection.Set(slots)
	v.turn = newConversation(v.vs)
	v.turn.Word = v.selection.ID
	slog.Info("wake words", "ours", len(ours), "slots", slots)

	v.vs.OnTimer = timer.Get().Event
	v.vs.OnSubscribed = func(subscribed bool) {
		if !subscribed {
			timer.Get().Forget()
		}
	}

	wakeword.Requested.Listen(v.Start)

	return v
}

func (v *Voice) Name() string { return "conversation" }

func (v *Voice) Handle(ctx context.Context, c *esphome.Conn, msg proto.Message) error {
	return v.vs.Handle(ctx, c, msg)
}

func (v *Voice) Run(ctx context.Context) error {
	v.turn.Run(ctx)
	return nil
}

func (v *Voice) Ready() bool { return v.vs.Subscribed() }

func (v *Voice) Start(slot int) { v.turn.Start(slot) }

func (v *Voice) Action() {
	if v.Stop() {
		return
	}

	// Home Assistant runs the first pipeline for a wake that reports no phrase.
	v.turn.Start(0)
}

func (v *Voice) Interrupt() {
	if v.turn.Phase() == phaseListening {
		slog.Debug("stop word ignored, the turn is listening")
		return
	}
	playing, _ := media.Get().Playing()
	if !speaker.Sound().Busy() && !timer.Get().Ringing() && !playing {
		slog.Debug("stop word ignored, nothing to stop")
		return
	}
	v.Stop()
}

func (v *Voice) Stop() bool {
	if timer.Get().Stop() {
		return true
	}

	if v.turn.Busy() {
		v.turn.Cancel()
		return true
	}

	if sound := speaker.Sound(); sound.Busy() {
		sound.Silence()
		return true
	}

	if playing, _ := media.Get().Playing(); playing {
		media.Get().Pause()
		return true
	}
	return false
}

// Home Assistant takes the echoed list as authoritative and reverts a slot missing from it.
func (v *Voice) OnWakeWord(load func(slots []string) []string) {
	v.vs.OnSetActiveWakeWords = func(ids []string) {
		accepted := load(ids)
		v.SetSlots(accepted)

		for slot, id := range accepted {
			if err := config.Set().Wake(slot).ID(id); err != nil {
				slog.Error("saving the wake word failed", "slot", slot+1, "err", err)
			}
		}
		if len(chosen(accepted)) != len(chosen(ids)) {
			slog.Warn("some wake words were refused", "asked", ids, "running", v.vs.ActiveWakeWords)
		}
	}
}

func (v *Voice) Slots() []string { return v.selection.Get() }
func (v *Voice) SetSlots(slots []string) {
	v.selection.Set(slots)
	v.vs.ActiveWakeWords = chosen(slots)
	slog.Info("wake words listening", "slots", slots)
}
func chosen(slots []string) []string { return wakeslots.Advertised(slots) }
