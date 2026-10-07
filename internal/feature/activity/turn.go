package activity

import sharedactivity "github.com/ygelfand/libcountertop/pkg/assistant/activity"

type Turn = sharedactivity.Turn
type Outcome = sharedactivity.Outcome

const (
	TurnEvent = "esphome.echolocal_turn"
	Version   = sharedactivity.Version
	Completed = sharedactivity.Completed
	Cancelled = sharedactivity.Cancelled
	Timeout   = sharedactivity.Timeout
	Failed    = sharedactivity.Failed
)
