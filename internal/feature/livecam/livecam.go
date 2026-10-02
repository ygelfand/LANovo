// Package livecam shares one lanovo-camera session between everything that wants the camera.
package livecam

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/mtkcamera"
)

const (
	FPS    = 30
	Wait   = 2 * time.Second
	Settle = 2 * time.Second
	depth  = 64
)

var ErrNoStill = errors.New("livecam: the helper could not make a picture")

var ErrRestarted = errors.New("livecam: the panel turned or the streams changed")

type Size struct{ Width, Height int }

// Sizes is each stream as it is encoded, turned to stand upright.
func Sizes() []Size { return sizesFor(Turn()) }

func sizesFor(q int) []Size {
	k := Saved()
	mw, mh := parseSize(k.MainSize)
	out := []Size{{mw, mh}}
	if k.SubOn {
		sw, sh := subFor(k)
		out = append(out, Size{sw, sh})
	}
	if q >= 0 && q%2 == 1 {
		for i := range out {
			out[i].Width, out[i].Height = out[i].Height, out[i].Width
		}
	}
	return out
}

// Turn is the quarter turns the helper draws the camera through.
func Turn() int {
	if board.Current().SoC == board.MediaTek {
		return 0
	}
	return turnFor(int(display.Get().Orientation()))
}

// mounted is the quarter turns clockwise that stand a Qualcomm board's frame up with the device at 0°.
const mounted = 3

func turnFor(device int) int {
	return (mounted - ((device/90)%4+4)%4 + 8) % 4
}

const Keyframe = 2

func Bitrate(w, h int) int { return bitrateFor(Saved(), w, h) }

type Picture struct {
	RGBA          []byte
	Width, Height int
}

type Session struct {
	stream *mtkcamera.Stream
	out    [2]chan mtkcamera.Frame
	config [2][]byte
	stills []chan mtkcamera.Frame
	users  int
	err    error
	done   chan struct{}
	opened time.Time
	turn   int
	shape  string
}

// Done closes when the session ends; Err says why.
func (s *Session) Done() <-chan struct{} { return s.done }

func (s *Session) Err() error {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if s.err == nil {
		return fmt.Errorf("livecam: the camera helper went away")
	}
	return s.err
}

type helperHub struct {
	mu  sync.Mutex
	cur *Session
}

var hub helperHub

// Join subscribes to stream at, opening the camera when nothing holds it.
func Join(at int) (*Session, <-chan mtkcamera.Frame, error) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	s, err := hub.session()
	if err != nil {
		return nil, nil, err
	}
	ch := make(chan mtkcamera.Frame, depth)
	if s.config[at] != nil {
		ch <- mtkcamera.Frame{Data: s.config[at], Config: true}
	}
	s.out[at] = ch
	return s, ch, nil
}

// Leave ends a Join.
func Leave(s *Session, at int) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	s.out[at] = nil
	hub.release(s)
}

// Still is a picture of the main stream.
func Still(within time.Duration) (Picture, error) {
	hub.mu.Lock()
	s, err := hub.session()
	if err != nil {
		hub.mu.Unlock()
		return Picture{}, err
	}
	young := Settle - time.Since(s.opened)
	hub.mu.Unlock()
	defer func() {
		hub.mu.Lock()
		hub.release(s)
		hub.mu.Unlock()
	}()
	if young > 0 {
		select {
		case <-time.After(young):
		case <-s.done:
			return Picture{}, s.Err()
		}
	}

	got := make(chan mtkcamera.Frame, 1)
	hub.mu.Lock()
	s.stills = append(s.stills, got)
	if len(s.stills) == 1 {
		err = s.stream.AskStill()
	}
	hub.mu.Unlock()
	if err != nil {
		return Picture{}, err
	}

	select {
	case f := <-got:
		if len(f.Data) == 0 {
			return Picture{}, ErrNoStill
		}
		return Picture{RGBA: f.Data, Width: f.Width, Height: f.Height}, nil
	case <-s.done:
		return Picture{}, s.Err()
	case <-time.After(within):
		return Picture{}, fmt.Errorf("livecam: no picture within %s", within)
	}
}

func (h *helperHub) session() (*Session, error) {
	if h.cur == nil {
		s, err := open()
		if err != nil {
			return nil, err
		}
		h.cur = s
	}
	h.cur.users++
	return h.cur, nil
}

func (h *helperHub) release(s *Session) {
	s.users--
	if s.users == 0 && h.cur == s {
		h.cur = nil
		s.stream.Close()
	}
}

func open() (*Session, error) {
	q := Turn()
	k := Saved()
	sz := sizesFor(q)
	cfg := mtkcamera.Config{Width: sz[0].Width, Height: sz[0].Height, FPS: FPS,
		Bitrate: bitrateFor(k, sz[0].Width, sz[0].Height), Keyframe: k.Keyframe, Params: Params(k), Turn: q, Mirror: board.Current().CameraMirror}
	if len(sz) > 1 {
		cfg.SubWidth, cfg.SubHeight = sz[1].Width, sz[1].Height
		cfg.SubBitrate = bitrateFor(k, sz[1].Width, sz[1].Height)
	}
	st, err := mtkcamera.Open(cfg)
	if err != nil {
		revive(err)
		return nil, err
	}
	s := &Session{stream: st, done: make(chan struct{}), opened: time.Now(), turn: q, shape: shapeOf(k)}
	go read(s)
	go follow(s)
	return s, nil
}

const reviveEvery = 30 * time.Second

var revived time.Time

func revive(cause error) {
	if time.Since(revived) < reviveEvery {
		return
	}
	revived = time.Now()
	slog.Warn("restarting the camera helper", "err", cause)
	if err := prop.Restart(prop.Local, helperService); err != nil {
		slog.Error("restarting the camera helper failed", "err", err)
	}
}

const helperService = "lanovo_camera"

func follow(s *Session) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
			if Turn() == s.turn && shapeOf(Saved()) == s.shape {
				continue
			}
			hub.mu.Lock()
			if hub.cur == s {
				s.err = ErrRestarted
				hub.cur = nil
				s.stream.Close()
			}
			hub.mu.Unlock()
			return
		}
	}
}

func read(s *Session) {
	defer close(s.done)
	for {
		f, err := s.stream.Next(Wait)
		if err != nil {
			if mtkcamera.Timeout(err) {
				err = fmt.Errorf("livecam: no frame from the camera within %s", Wait)
			}
			hub.mu.Lock()
			if s.err == nil {
				s.err = err
			}
			if hub.cur == s {
				hub.cur = nil
				s.stream.Close()
			}
			hub.mu.Unlock()
			return
		}

		hub.mu.Lock()
		if f.Still {
			for _, ch := range s.stills {
				ch <- f
			}
			s.stills = nil
			hub.mu.Unlock()
			continue
		}
		at := 0
		if f.Sub {
			at = 1
		}
		if f.Config {
			s.config[at] = f.Data
		}
		if ch := s.out[at]; ch != nil {
			select {
			case ch <- f:
			default:
			}
		}
		hub.mu.Unlock()
	}
}
