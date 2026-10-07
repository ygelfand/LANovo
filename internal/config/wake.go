package config

import "github.com/ygelfand/libcountertop/pkg/say"

import (
	"fmt"
	"slices"
)

func errSlot(n int) error { return fmt.Errorf("config: wake slot %d", n) }

// Wake is the wake word configuration, indexed by Home Assistant's wake word slot.
type Wake struct {
	Words []WakeWord `json:"words"`

	// Stop is the device's own word for interrupting what it is saying. It is not one of the slots
	// above: Home Assistant does not choose it, and it opens no pipeline.
	Stop Stop `json:"stop"`
}

// Stop is the interrupting word.
type Stop struct {
	// Threshold is the score it has to reach, on its own scale. At StopOff the word is not listened
	// for at all: the model is left unloaded, so switching it off costs nothing rather than costing
	// a comparison. There is no separate switch because this is one.
	Threshold float64 `json:"threshold"`
}

const (
	// StopOff is a threshold no score can reach, which is how the word is turned off.
	StopOff = 1.0

	// Above the 0.5 the model is calibrated for, below where spoken attempts land.
	DefaultStopThreshold = 0.7
)

func defaultStop() Stop { return Stop{Threshold: DefaultStopThreshold} }

func defaultWake() Wake { return Wake{Stop: defaultStop()} }

// Listening reports whether the stop word is being listened for.
func (s Stop) Listening() bool { return s.Threshold < StopOff }

// WakeWord is one slot: which wake word listens there and how it behaves when it fires. An empty ID
// is the slot switched off, which is also how detection is turned off altogether.
type WakeWord struct {
	ID string `json:"id"`

	// Threshold is the score a detection has to reach. Per word, because models disagree on scale.
	Threshold float64 `json:"threshold"`

	// Tone is the sound the slot makes when it fires, sharing the device's chimes.
	Tone Chime `json:"tone"`

	// Delivery is how the reply from this slot's pipeline reaches the device.
	Delivery Delivery `json:"delivery"`

	// FollowUp is seconds to listen after a reply, zero to only do it when Home Assistant asks.
	FollowUp int `json:"follow_up"`

	// Buffer is milliseconds of a streamed reply to collect before playing any of it.
	Buffer int `json:"buffer"`

	// Seconds before giving up. Listening holds the microphone open and Home Assistant normally ends
	// it, so that one is a backstop; thinking holds only the screen, and a model can take a minute.
	MaxListen int `json:"max_listen"`
	MaxThink  int `json:"max_think"`

	// Recordings is how many of this slot's turns to keep the audio of on disk. Zero keeps none.
	Recordings int `json:"recordings"`

	Look Look `json:"look"`
}

const (
	DefaultThreshold = 0.85
	DefaultTone      = ChimeChirp
	DefaultDelivery  = DeliveryWhole

	DefaultMaxListen = 15
	DefaultMaxThink  = 90

	// Zero is no follow-up unless Home Assistant asks for one.
	DefaultFollowUp = 0

	// Home Assistant paces itself to stay 384 ms ahead, so holding that much consumes the whole
	// lead: measured, 384 gave 8 seams in a 13 second reply and 650 gave one.
	DefaultBuffer = 650
)

// DefaultWakeWord is a slot nobody has set: switched off, and everything else ready for when it is.
func DefaultWakeWord() WakeWord {
	return WakeWord{
		Threshold: DefaultThreshold,
		Tone:      DefaultTone,
		Delivery:  DefaultDelivery,
		FollowUp:  DefaultFollowUp,
		Buffer:    DefaultBuffer,
		MaxListen: DefaultMaxListen,
		MaxThink:  DefaultMaxThink,
		Look:      Look{Place: LookPanel},
	}
}

// Slot is one wake word slot, or an unset one with the defaults in it.
func (w Wake) Slot(n int) WakeWord {
	if n < 0 || n >= len(w.Words) {
		return DefaultWakeWord()
	}
	return w.Words[n]
}

// Slots is the first n slots, one entry each whether or not any has been set.
func (w Wake) Slots(n int) []WakeWord {
	out := make([]WakeWord, n)
	for i := range out {
		out[i] = w.Slot(i)
	}
	return out
}

// IDs is the wake word in each of the first n slots, empty where a slot is off. It is what Home
// Assistant is told is active, so the positions matter and the gaps are kept.
func (w Wake) IDs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = w.Slot(i).ID
	}
	return out
}

// StopWriter is the stop word, which belongs to no slot.
type StopWriter struct{ st *Store }

func (w StopWriter) Threshold(v float64) error {
	return w.st.Update(func(c *Config) { c.Wake.Stop.Threshold = v })
}

type WakeWriter struct {
	st   *Store
	slot int
}

func (w WakeWriter) ID(v string) error {
	return w.word(func(word *WakeWord) { word.ID = v })
}

func (w WakeWriter) Threshold(v float64) error {
	return w.word(func(word *WakeWord) { word.Threshold = v })
}

func (w WakeWriter) Tone(v Chime) error {
	return w.word(func(word *WakeWord) { word.Tone = v })
}

func (w WakeWriter) Delivery(v Delivery) error {
	return w.word(func(word *WakeWord) { word.Delivery = v })
}

func (w WakeWriter) FollowUp(seconds int) error {
	return w.word(func(word *WakeWord) { word.FollowUp = seconds })
}

func (w WakeWriter) Buffer(ms int) error {
	return w.word(func(word *WakeWord) { word.Buffer = ms })
}

func (w WakeWriter) MaxListen(seconds int) error {
	return w.word(func(word *WakeWord) { word.MaxListen = seconds })
}

func (w WakeWriter) MaxThink(seconds int) error {
	return w.word(func(word *WakeWord) { word.MaxThink = seconds })
}

func (w WakeWriter) Recordings(count int) error {
	return w.word(func(word *WakeWord) { word.Recordings = count })
}

// word grows the list to reach the slot, so slot 1 can be set on a device where slot 0 never was.
// The slots invented along the way get the defaults rather than zeros.
func (w WakeWriter) word(f func(*WakeWord)) error {
	if w.slot < 0 {
		return errSlot(w.slot)
	}
	return w.st.Update(func(c *Config) {
		words := slices.Clone(c.Wake.Words)
		for len(words) <= w.slot {
			words = append(words, DefaultWakeWord())
		}
		f(&words[w.slot])
		c.Wake.Words = words
	})
}

// Delivery is how a spoken reply reaches the device. It is per slot because a local pipeline and a
// cloud one differ in how long the audio takes to start, so the trade between starting sooner and
// not gapping is not the same for both.
type Delivery string

const (
	// DeliveryWhole fetches the reply from the url Home Assistant serves it at. It cannot gap and it
	// says when the audio has ended, which the stream does not.
	DeliveryWhole Delivery = "whole"

	// DeliveryStream takes the reply over the API as it is generated. It starts sooner and it can
	// gap: the chunks arrive at about the rate they play, so any hiccup splices silence into a word.
	DeliveryStream Delivery = "stream"
)

// Label is how the setting is shown.
func (d Delivery) Label() string {
	switch d {
	case DeliveryWhole:
		return say.T("delivery.whole")
	case DeliveryStream:
		return say.T("delivery.stream")
	}
	return string(d)
}

// Deliveries is how a reply can arrive, in the order it is offered.
func Deliveries() []Delivery { return []Delivery{DeliveryWhole, DeliveryStream} }
