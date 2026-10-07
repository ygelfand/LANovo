package voice

import (
	"github.com/ygelfand/libcountertop/pkg/assistant/turn"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

type Phase = turn.Phase
type Showing = turn.Showing

const (
	Idle      = turn.Idle
	Listening = turn.Listening
	Thinking  = turn.Thinking
	Replying  = turn.Replying
)

var Shown hook.Hook[Showing]

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
