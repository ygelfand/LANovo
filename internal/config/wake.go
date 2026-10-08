package config

import (
	"fmt"
	"slices"

	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

func errSlot(n int) error { return fmt.Errorf("config: wake slot %d", n) }

type Wake = schema.Wake
type Stop = schema.Stop
type WakeWord = schema.WakeWord

const (
	StopOff              = schema.StopOff
	DefaultStopThreshold = schema.DefaultStopThreshold
	DefaultThreshold     = schema.DefaultThreshold
	DefaultEffect        = schema.DefaultEffect
	DefaultTone          = schema.DefaultTone
	DefaultDelivery      = schema.DefaultDelivery
	DefaultMaxListen     = schema.DefaultMaxListen
	DefaultMaxThink      = schema.DefaultMaxThink
	DefaultFollowUp      = schema.DefaultFollowUp
	DefaultBuffer        = schema.DefaultBuffer
)

var defaultWake = schema.DefaultWake
var DefaultWakeWord = schema.DefaultWakeWord

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

type Delivery = schema.Delivery

const (
	DeliveryWhole  = schema.DeliveryWhole
	DeliveryStream = schema.DeliveryStream
)

var Deliveries = schema.Deliveries
