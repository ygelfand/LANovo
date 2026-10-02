package config

import "github.com/ygelfand/LANovo/internal/lib/say"

type Look struct {
	Place      LookPlace `json:"place"`
	Listening  string    `json:"listening"`
	Waiting    string    `json:"waiting"`
	Responding string    `json:"responding"`
}

type LookPlace string

const (
	LookPanel LookPlace = "panel"
	LookFull  LookPlace = "full"
)

func (p LookPlace) Label() string {
	switch p {
	case LookFull:
		return say.T("look.full")
	}
	return say.T("look.panel")
}

func LookPlaces() []LookPlace { return []LookPlace{LookPanel, LookFull} }

type Stage string

const (
	StageListening  Stage = "listening"
	StageWaiting    Stage = "waiting"
	StageResponding Stage = "responding"
)

func Stages() []Stage { return []Stage{StageListening, StageWaiting, StageResponding} }

func (s Stage) Label() string { return say.T("look.stage." + string(s)) }

func (s Stage) Source() Source {
	switch s {
	case StageListening:
		return SourceMic
	case StageResponding:
		return SourceSpeaker
	}
	return SourceBoth
}

func (l Look) Kind(s Stage) string {
	switch s {
	case StageListening:
		return l.Listening
	case StageWaiting:
		return l.Waiting
	case StageResponding:
		return l.Responding
	}
	return ""
}

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
