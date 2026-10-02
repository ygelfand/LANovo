package voice

import "github.com/ygelfand/LANovo/internal/lib/hook"

// Phase is what a turn is doing, for whatever shows it. The conversation's own phase is its
// business; this is the part of it worth looking at.
type Phase int

const (
	Idle Phase = iota
	Listening
	Thinking
	Replying
)

func (p Phase) String() string {
	switch p {
	case Listening:
		return "listening"
	case Thinking:
		return "thinking"
	case Replying:
		return "replying"
	}
	return "idle"
}

// Showing is a turn as something to look at: the phase, the slot whose assistant it belongs to, and
// the words either side of it.
//
// Said is what Home Assistant heard, which arrives when the talker stops rather than while they are
// speaking — the device does not transcribe, so this is the earliest the words exist. Reply is what
// it is about to say, and arrives before the audio does, which is what makes revealing it in time
// with the speech possible at all.
type Showing struct {
	Phase Phase
	Slot  int

	Said  string
	Reply string
}

// Shown carries every phase change. This device answers on a screen where echolocal answers on a
// ring, so what the turn does is said once here and drawn somewhere else.
var Shown hook.Hook[Showing]

// shown is the internal phase as the thing outside sees it.
func shown(p phase) Phase {
	switch p {
	case phaseListening:
		return Listening
	case phaseThinking:
		return Thinking
	case phaseReplying:
		return Replying
	}
	return Idle
}
