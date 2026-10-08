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
