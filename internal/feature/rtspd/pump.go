package rtspd

import (
	"log/slog"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/rtsp"
)

const linger = 15 * time.Second

var names = [...]string{"main", "sub"}

func streamName(i int) string {
	if i < 0 || i >= len(names) {
		return ""
	}
	return names[i]
}

type pump struct {
	at      int
	stream  *rtsp.Stream
	allowed func() bool
	linger  time.Duration

	mu      sync.Mutex
	running bool
	wanted  time.Time
	stop    chan struct{}
}

func (p *pump) demand() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.wanted = time.Now()
	if p.running || !p.allowed() {
		return
	}
	p.running = true
	p.stop = make(chan struct{})
	go p.run(p.stop)
}

func (p *pump) halt() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running && p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
}

func (p *pump) idle() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	wait := p.linger
	if wait == 0 {
		wait = linger
	}
	return p.stream.Viewers() == 0 && time.Since(p.wanted) > wait
}

func (p *pump) run(stop chan struct{}) {
	defer func() {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()
	if err := p.helper(stop); err != nil {
		slog.Warn("the rtsp stream stopped", "stream", streamName(p.at), "err", err)
	}
}
