// Package voice is a turn: hearing a wake word, listening, waiting for an answer and playing it.
//
// Two halves. The conversation is a state machine on its own goroutine, so a button press, a pipeline
// event and a timeout cannot race each other. Around it sits the voice satellite, which is what Home
// Assistant talks to.
//
// Nothing here has an entity: a turn is not a setting and not a reading.
package voice

import (
	"context"
	"github.com/ygelfand/libcountertop/pkg/inference/wakeslots"
	"log/slog"
	"sync"

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

// Features is what the device claims it can do with a voice pipeline. Announce and
// StartConversation go together and both need a media player, which this device has.
const Features = esphome.DefaultVoiceFeatures |
	esphome.FeatureSpeaker |
	esphome.FeatureAnnounce |
	esphome.FeatureStartConversation |
	esphome.FeatureTimers

type Voice struct {
	vs   *esphome.VoiceSatellite
	turn *conversation

	// slots is the selection by slot, empty where a slot is off. What Home Assistant is told is this
	// with the gaps taken out, because it reads that list as a set; the engine is given the list
	// itself, because a word in slot 2 has to load into slot 2 or it arms the wrong pipeline.
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

// build makes the satellite and the conversation together: the satellite's callbacks are the
// conversation's inputs, and the conversation answers back through it.
//
// What the device can hear is not stored — it is worked out on every configuration request, because
// models arrive and are deleted while the device runs.
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

// Handle is the satellite's own protocol messages, which have no entity to arrive through.
func (v *Voice) Handle(ctx context.Context, c *esphome.Conn, msg proto.Message) error {
	return v.vs.Handle(ctx, c, msg)
}

// Run owns the conversation until ctx is cancelled. Nothing happens on a wake word until it is
// running.
func (v *Voice) Run(ctx context.Context) error {
	v.turn.Run(ctx)
	return nil
}

// Ready reports whether Home Assistant has a voice pipeline listening. Wake detection runs before
// that happens, but nothing can be done with a detection until it does, so this is what the device
// shows on the ring while it comes up.
func (v *Voice) Ready() bool { return v.vs.Subscribed() }

// Start asks for a turn as if that slot's wake word had fired, which is how detection and the
// buttons both reach a pipeline. What that means from the phase the conversation is already in is
// the conversation's decision, not the caller's.
func (v *Voice) Start(slot int) { v.turn.Start(slot) }

// Action is the action button: it gives up on whatever is happening, or starts something if nothing
// is. Cancelling is the more useful half — it is the way out of a turn that is waiting on a pipeline
// that is not going to answer.
func (v *Voice) Action() {
	// Anything audible is what the press meant. Asking a question is what the button is for when the
	// device is doing nothing; while it is talking or playing, reaching for it means make it stop.
	if v.Stop() {
		return
	}

	// No wake word, so no slot to pair with: the first pipeline is the one Home Assistant falls back
	// to for anything that reports no phrase.
	v.turn.Start(0)
}

// Interrupt is the stop word.
//
// It only acts while the device is making a sound. "Stop" is an ordinary word: somebody halfway through
// "stop the timer" is talking to Home Assistant, not to the device, and cutting their turn off there
// would be worse than not listening for it at all. Nothing is playing then, so there is nothing the word
// could sensibly mean.
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

// Stop ends whatever the device is doing audibly, and reports whether there was anything to end.
//
// One ladder, because there is one meaning: a turn is cancelled, a sound is silenced, a track is
// stopped. The action button falls through to starting a turn when it returns false; a stop word has
// nothing to fall through to and simply does nothing.
func (v *Voice) Stop() bool {
	// Before the turn, because a timer ringing over one is what the person is reaching for.
	if timer.Get().Stop() {
		return true
	}

	if v.turn.Busy() {
		v.turn.Cancel()
		return true
	}

	// An announcement outside a turn: nothing is listening, but something is playing.
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

// OnWakeWord is called when Home Assistant changes the selection, so the engine can follow. It is
// given every slot: load reports which of them it accepted, and only those are echoed back as
// active, because Home Assistant takes the echo as authoritative and reverts a slot whose word is
// missing from it. A slot the device will not run therefore reverts in the interface rather than
// sitting there looking armed.
func (v *Voice) OnWakeWord(load func(slots []string) []string) {
	v.vs.OnSetActiveWakeWords = func(ids []string) {
		accepted := load(ids)
		v.SetSlots(accepted)

		// By slot, from the positional list: a word refused in slot 1 must not shift slot 2 up into
		// its place, which would arm the wrong pipeline for it.
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

// ActiveWakeWords is what the device is advertising as listening, by slot.
// Slots is the selection by slot, which is what the engine loads from.
func (v *Voice) Slots() []string { return v.selection.Get() }
func (v *Voice) SetSlots(slots []string) {
	v.selection.Set(slots)
	v.vs.ActiveWakeWords = chosen(slots)
	slog.Info("wake words listening", "slots", slots)
}
func chosen(slots []string) []string { return wakeslots.Advertised(slots) }
