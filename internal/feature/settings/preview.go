package settings

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/lib/surface"
	"github.com/ygelfand/LANovo/internal/ui"
)

const (
	camIdle  = 750 * time.Millisecond
	camHold  = 5 * time.Second
	camLayer = 900
	camZ     = 6
)

var cam struct {
	mu    sync.Mutex
	pages []*shell.Page
	want  time.Time
	live  bool
	shown bool
	bad   time.Time
	said  string
	box   ui.Rect
}

var (
	camLeft   = make(chan shell.Change, 1)
	camListen sync.Once
)

func camPage(p *shell.Page) *shell.Page {
	camListen.Do(func() {
		shell.Get().Changed.Listen(func(c shell.Change) {
			if !camOnScreen() {
				select {
				case camLeft <- c:
				default:
				}
			}
		})
	})
	cam.mu.Lock()
	defer cam.mu.Unlock()
	cam.pages = append(cam.pages, p)
	return p
}

func camOnScreen() bool {
	cam.mu.Lock()
	pages := cam.pages
	cam.mu.Unlock()
	for _, p := range pages {
		if shell.Get().Visible(p) {
			return true
		}
	}
	return false
}

func camStream() int { return len(livecam.Sizes()) - 1 }

func camWant(box ui.Rect) bool {
	cam.mu.Lock()
	defer cam.mu.Unlock()
	cam.want, cam.box = time.Now(), box
	if !cam.live && time.Since(cam.bad) > camHold {
		cam.live = true
		go camRun()
	}
	return cam.shown
}

func camWatched() (ui.Rect, bool) {
	cam.mu.Lock()
	defer cam.mu.Unlock()
	return cam.box, time.Since(cam.want) < camIdle
}

func camRun() {
	err := camPlay()
	cam.mu.Lock()
	cam.live, cam.shown = false, false
	if errors.Is(err, livecam.ErrRestarted) {
		err = nil
	}
	if err != nil {
		cam.bad = time.Now()
		if err.Error() != cam.said {
			slog.Warn("the camera preview stopped", "err", err)
		}
		cam.said = err.Error()
	}
	cam.mu.Unlock()
	shell.Get().Redraw()
}

func camPlace(c *surface.Client, box ui.Rect, w, h int, visible bool) error {
	fit := camFit(w, h, box)
	fbW, fbH := display.Get().Native()
	x, y, m, _ := display.Get().Orientation().Place(fbW, fbH, display.Rect{X: fit.X, Y: fit.Y, W: fit.W, H: fit.H}, w, h)
	return c.VideoPlace(camLayer, x, y, m, camZ, visible)
}

func camPlay() error {
	c := display.Get().Helper()
	if c == nil {
		return nil
	}
	at := camStream()
	sz := livecam.Sizes()[at]
	s, frames, err := livecam.Join(at)
	if err != nil {
		return err
	}
	defer livecam.Leave(s, frames)

	if err := c.VideoOpen(camLayer, surface.AVC, sz.Width, sz.Height, camZ, 0, ""); err != nil {
		return err
	}
	defer c.VideoClose(camLayer)

	var config []byte
	keyed := false
	placed := ui.Rect{}
	orient := display.Get().Orientation()
	check := time.NewTicker(250 * time.Millisecond)
	defer check.Stop()
	select {
	case <-camLeft:
	default:
	}

	for {
		select {
		case <-s.Done():
			return s.Err()
		case <-camLeft:
			return nil
		case <-check.C:
			box, watched := camWatched()
			if !watched {
				return nil
			}
			shell.Get().Redraw()
			if o := display.Get().Orientation(); keyed && (box != placed || o != orient) {
				placed, orient = box, o
				if err := camPlace(c, box, sz.Width, sz.Height, true); err != nil {
					return err
				}
			}
		case f := <-frames:
			if f.Config {
				config = f.Data
				continue
			}
			data := f.Data
			if !keyed {
				if !f.Key {
					continue
				}
				data = append(append([]byte{}, config...), f.Data...)
			}
			if err := camFeed(c, f.PTS, data, s.Done()); err != nil {
				return err
			}
			if err := c.VideoClock(camLayer, f.PTS-camLag, true); err != nil {
				return err
			}
			if !keyed {
				keyed = true
				placed, _ = camWatched()
				if err := camPlace(c, placed, sz.Width, sz.Height, true); err != nil {
					return err
				}
				cam.mu.Lock()
				cam.shown = true
				cam.mu.Unlock()
				shell.Get().Redraw()
			}
		}
	}
}

const (
	camRetry = 20 * time.Millisecond
	camLag   = 200 * time.Millisecond
)

func camFeed(c *surface.Client, pts time.Duration, data []byte, done <-chan struct{}) error {
	for {
		_, err := c.VideoSample(camLayer, pts, 0, data)
		if !errors.Is(err, surface.Full) {
			return err
		}
		select {
		case <-done:
			return nil
		case <-time.After(camRetry):
		}
	}
}

func camFit(w, h int, box ui.Rect) ui.Rect {
	dw, dh := box.W, box.W*h/max(1, w)
	if dh > box.H {
		dw, dh = box.H*w/max(1, h), box.H
	}
	return ui.Rect{X: box.X + (box.W-dw)/2, Y: box.Y + (box.H-dh)/2, W: dw, H: dh}
}
