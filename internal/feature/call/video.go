package call

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/mtkcamera"
	"github.com/ygelfand/LANovo/internal/lib/rtc"
	"github.com/ygelfand/LANovo/internal/lib/safe"
	"github.com/ygelfand/LANovo/internal/lib/surface"
)

const (
	remoteLayer = 920
	selfLayer   = 921
	remoteZ     = -2
	selfZ       = -1
	videoLag    = 200 * time.Millisecond
	feedRetry   = 20 * time.Millisecond
	feedGiveUp  = 200 * time.Millisecond
	rejoinWait  = 500 * time.Millisecond
	selfShare   = 0.28
	selfMargin  = 0.03
	sideShare   = 0.25
	belowShare  = 0.3
	turnCheck   = 250 * time.Millisecond
)

type Layout struct {
	Remote  display.Rect
	Picture display.Rect
	Self    display.Rect
	Panel   display.Rect
	Side    bool
}

func arrange(vw, vh int, remote, own livecam.Size) Layout {
	full := fullScreen(vw, vh)
	m := int(float64(min(vw, vh)) * selfMargin)
	if remote.Width > 0 && remote.Height > 0 {
		r := fit(full, remote.Width, remote.Height)
		switch {
		case vh > vw && vh-r.H >= int(float64(vh)*belowShare):
			pic := display.Rect{W: vw, H: r.H}
			panel := display.Rect{Y: r.H, W: vw, H: vh - r.H}
			self := display.Rect{X: panel.W / 2, Y: panel.Y + m, W: panel.W/2 - m, H: panel.H / 2}
			return Layout{Remote: pic, Picture: pic, Panel: panel, Side: true, Self: fitOwn(self, own, true)}
		case vw > vh && vw-r.W >= int(float64(vw)*sideShare):
			pic := display.Rect{W: r.W, H: vh}
			panel := display.Rect{X: r.W, W: vw - r.W, H: vh}
			self := display.Rect{X: panel.X + m, Y: panel.H / 5, W: panel.W - 2*m, H: panel.H * 2 / 5}
			return Layout{Remote: pic, Picture: pic, Panel: panel, Side: true, Self: fitOwn(self, own, false)}
		}
		return Layout{Remote: full, Picture: r, Panel: full, Self: corner(vw, vh)}
	}
	return Layout{Remote: full, Picture: full, Panel: full, Self: corner(vw, vh)}
}

func fitOwn(box display.Rect, own livecam.Size, right bool) display.Rect {
	if own.Width == 0 || own.Height == 0 {
		return box
	}
	r := fit(box, own.Width, own.Height)
	if right {
		r.X = box.X + box.W - r.W
	}
	r.Y = box.Y
	return r
}

func (c *Calls) layout(vw, vh int) Layout {
	c.mu.Lock()
	var remote, own livecam.Size
	if c.now != nil {
		remote, own = c.now.remote, c.now.own
	}
	c.mu.Unlock()
	return arrange(vw, vh, remote, own)
}

func Arrange(vw, vh int) Layout { return Get().layout(vw, vh) }

func cameraStream() (int, livecam.Size, bool) {
	sizes := livecam.Sizes()
	if len(sizes) == 0 {
		return 0, livecam.Size{}, false
	}
	if config.Get().Call.Stream == config.CallSub && len(sizes) > 1 {
		return 1, sizes[1], true
	}
	return 0, sizes[0], true
}

type camera struct {
	at    int
	sized func(livecam.Size)
}

func (c camera) Frames() (<-chan rtc.Frame, func()) {
	out := make(chan rtc.Frame, 8)
	ctx, stop := context.WithCancel(context.Background())
	safe.Go("call camera", func() {
		defer close(out)
		var size livecam.Size
		for ctx.Err() == nil {
			s, frames, err := livecam.Join(c.at)
			if err != nil {
				slog.Warn("call camera", "err", err)
			} else {
				if sizes := livecam.Sizes(); c.at < len(sizes) && sizes[c.at] != size {
					size = sizes[c.at]
					if c.sized != nil {
						c.sized(size)
					}
				}
				pump(ctx, s, frames, out)
				livecam.Leave(s, frames)
			}
			select {
			case <-ctx.Done():
			case <-time.After(rejoinWait):
			}
		}
	})
	return out, stop
}

func pump(ctx context.Context, s *livecam.Session, frames <-chan mtkcamera.Frame, out chan<- rtc.Frame) {
	var config []byte
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.Done():
			return
		case f := <-frames:
			if f.Config {
				config = append([]byte(nil), f.Data...)
				continue
			}
			data := f.Data
			if f.Key && config != nil {
				data = append(append([]byte(nil), config...), f.Data...)
			}
			select {
			case out <- rtc.Frame{Data: data, PTS: f.PTS, Key: f.Key}:
			default:
			}
		}
	}
}

type layer struct {
	id   uint32
	z    int
	size livecam.Size
	box  func(vw, vh int) display.Rect

	mu     sync.Mutex
	open   bool
	shown  bool
	hidden bool
	frames uint64
	failed error
}

type LayerStats struct {
	Open   bool
	Width  int
	Height int
	Frames uint64
	Failed error
}

func (l *layer) stats() LayerStats {
	if l == nil {
		return LayerStats{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return LayerStats{Open: l.open, Width: l.size.Width, Height: l.size.Height, Frames: l.frames, Failed: l.failed}
}

func (l *layer) Show(f rtc.Frame) {
	c := display.Get().Helper()
	if c == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.open {
		if !f.Key || l.size.Width == 0 {
			return
		}
		if err := c.VideoOpen(l.id, surface.AVC, l.size.Width, l.size.Height, l.z, 0, ""); err != nil {
			slog.Warn("call video open", "layer", l.id, "width", l.size.Width, "height", l.size.Height, "err", err)
			l.failed = err
			return
		}
		slog.Info("call video opened", "layer", l.id, "width", l.size.Width, "height", l.size.Height)
		l.open, l.failed = true, nil
	}
	if err := feed(c, l.id, f); err != nil {
		slog.Debug("call video feed", "layer", l.id, "err", err)
		l.failed = err
		return
	}
	l.frames++
	c.VideoClock(l.id, f.PTS-videoLag, true)
	if !l.shown {
		l.shown = true
		l.place(c)
	}
}

func (l *layer) resize(sz livecam.Size) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if sz == l.size {
		return
	}
	l.size = sz
	if l.open {
		if c := display.Get().Helper(); c != nil {
			c.VideoClose(l.id)
		}
		l.open, l.shown = false, false
	}
}

func (l *layer) place(c *surface.Client) {
	o := display.Get().Orientation()
	fw, fh := display.Get().Native()
	vw, vh := o.Size(fw, fh)
	at := fit(l.box(vw, vh), l.size.Width, l.size.Height)
	x, y, m, _ := o.Place(fw, fh, at, l.size.Width, l.size.Height)
	if err := c.VideoPlace(l.id, x, y, m, l.z, !l.hidden); err != nil {
		slog.Debug("call video place", "layer", l.id, "err", err)
	}
}

func (l *layer) hide(on bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hidden = on
	if c := display.Get().Helper(); c != nil && l.shown {
		l.place(c)
	}
}

func (l *layer) reorient() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if c := display.Get().Helper(); c != nil && l.shown {
		l.place(c)
	}
}

func (l *layer) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.open {
		return
	}
	if c := display.Get().Helper(); c != nil {
		c.VideoClose(l.id)
	}
	l.open, l.shown = false, false
}

func feed(c *surface.Client, id uint32, f rtc.Frame) error {
	var flags uint32
	if f.Key {
		flags = surface.SampleKey
	}
	give := time.Now().Add(feedGiveUp)
	for {
		_, err := c.VideoSample(id, f.PTS, flags, f.Data)
		if !errors.Is(err, surface.Full) || time.Now().After(give) {
			return err
		}
		time.Sleep(feedRetry)
	}
}

func fit(box display.Rect, w, h int) display.Rect {
	dw, dh := box.W, box.W*h/max(1, w)
	if dh > box.H {
		dw, dh = box.H*w/max(1, h), box.H
	}
	return display.Rect{X: box.X + (box.W-dw)/2, Y: box.Y + (box.H-dh)/2, W: dw, H: dh}
}

func fullScreen(vw, vh int) display.Rect { return display.Rect{W: vw, H: vh} }

func corner(vw, vh int) display.Rect {
	w := int(float64(vw) * selfShare)
	h := int(float64(vh) * selfShare)
	m := int(float64(min(vw, vh)) * selfMargin)
	return display.Rect{X: vw - w - m, Y: vh - h - m, W: w, H: h}
}

type pictures struct {
	remote *layer
	self   *layer
	stop   func()
}

func (c *Calls) newPictures(remote livecam.Size) *pictures {
	return &pictures{remote: &layer{id: remoteLayer, z: remoteZ, size: remote, box: func(vw, vh int) display.Rect { return c.layout(vw, vh).Remote }}}
}

func (p *pictures) start(ctx context.Context, c *Calls, s *session, at int, own livecam.Size, visible bool) {
	p.self = &layer{id: selfLayer, z: selfZ, size: own, hidden: !visible, box: func(vw, vh int) display.Rect { return c.layout(vw, vh).Self }}
	self := p.self
	frames, stop := camera{at: at, sized: func(sz livecam.Size) { c.ownSized(s, sz) }}.Frames()
	p.stop = stop
	safe.Go("call self view", func() {
		for f := range frames {
			self.Show(f)
		}
	})
	safe.Go("call video turn", func() { p.follow(ctx) })
}

func (p *pictures) follow(ctx context.Context) {
	tick := time.NewTicker(turnCheck)
	defer tick.Stop()
	was := display.Get().Orientation()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if now := display.Get().Orientation(); now != was {
				was = now
				p.replace()
			}
		}
	}
}

func (p *pictures) replace() {
	if p == nil {
		return
	}
	p.remote.reorient()
	p.self.reorient()
}

func (p *pictures) showRemote(on bool) {
	if p == nil || p.remote == nil {
		return
	}
	p.remote.hide(!on)
}

func (p *pictures) showSelf(on bool) {
	if p == nil || p.self == nil {
		return
	}
	p.self.hide(!on)
}

func (p *pictures) close() {
	if p == nil {
		return
	}
	if p.stop != nil {
		p.stop()
	}
	if p.self != nil {
		p.self.close()
	}
	if p.remote != nil {
		p.remote.close()
	}
}
