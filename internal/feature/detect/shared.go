package detect

import sharedengine "github.com/ygelfand/libcountertop/pkg/inference/detect"

type Engine struct {
	*sharedengine.Engine
	queue *sharedengine.Queue
}

func New(slots int) *Engine {
	q := sharedengine.NewQueue(8)
	return &Engine{Engine: sharedengine.New(slots, q), queue: q}
}
func (e *Engine) Feed(samples []int16) { e.queue.Feed(samples) }

const (
	Hold           = sharedengine.Hold
	Refractory     = sharedengine.Refractory
	NearMiss       = sharedengine.NearMiss
	NearMissSettle = sharedengine.NearMissSettle
)
