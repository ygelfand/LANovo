package chromecast

import (
	"math"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/volume"
)

type loudness struct {
	mu  sync.Mutex
	svc *cast.Service

	wants chan int
}

func newLoudness() *loudness {
	l := &loudness{wants: make(chan int, 1)}
	safe.Go("cast volume", l.run)
	volume.Get().Changed.Listen(l.changed)
	return l
}

func (l *loudness) asked(level float64, muted bool) {
	want := int(math.Round(level * 100))
	if muted {
		want = 0
	}

	select {
	case <-l.wants:
	default:
	}
	l.wants <- want
}

func (l *loudness) run() {
	for want := range l.wants {
		volume.Get().Set(config.StreamMedia, want)
	}
}

func (l *loudness) changed(ch volume.Change) {
	if ch.Stream != config.StreamMedia {
		return
	}

	l.mu.Lock()
	svc := l.svc
	l.mu.Unlock()

	if svc != nil {
		svc.Report(float64(ch.Level)/100, false)
	}
}

func (l *loudness) serve(svc *cast.Service) {
	l.mu.Lock()
	l.svc = svc
	l.mu.Unlock()

	if svc != nil {
		svc.Report(float64(volume.Get().Level(config.StreamMedia))/100, false)
	}
}
