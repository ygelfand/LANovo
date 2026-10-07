package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Look = schema.Look
type LookPlace = schema.LookPlace
type Stage = schema.Stage

const (
	LookPanel       = schema.LookPanel
	LookFull        = schema.LookFull
	StageListening  = schema.StageListening
	StageWaiting    = schema.StageWaiting
	StageResponding = schema.StageResponding
)

var LookPlaces = schema.LookPlaces
var Stages = schema.Stages

func (w WakeWriter) Place(v LookPlace) error {
	return w.word(func(ww *WakeWord) { ww.Look.Place = v })
}

func (w WakeWriter) Visual(s Stage, kind string) error {
	return w.word(func(ww *WakeWord) {
		switch s {
		case StageListening:
			ww.Look.Listening = kind
		case StageWaiting:
			ww.Look.Waiting = kind
		case StageResponding:
			ww.Look.Responding = kind
		}
	})
}
