package voice

import (
	"github.com/ygelfand/libcountertop/pkg/assistant/turn"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

var Shown hook.Hook[turn.Showing]
