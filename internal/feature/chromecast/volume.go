package chromecast

import (
	"math"
	"sync"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/libcountertop/pkg/media/cast"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

// loudness is cast as a client of the media volume: it passes on what senders set and reports what the volume is.
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

// asked is a sender's volume, arriving under the service's lock, which the volume's listeners report back through.
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

// serve points it at the running service, or at none, and tells a new one where the volume is.
func (l *loudness) serve(svc *cast.Service) {
	l.mu.Lock()
	l.svc = svc
	l.mu.Unlock()

	if svc != nil {
		svc.Report(float64(volume.Get().Level(config.StreamMedia))/100, false)
	}
}
