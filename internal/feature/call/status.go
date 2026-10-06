package call

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/lib/rtc"
)

const (
	kindStatus = "status"
	kindSize   = "size"
)

type size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type status struct {
	Muted     bool `json:"muted"`
	CameraOff bool `json:"camera_off"`
}

func (c *Calls) tell(s *session) {
	c.mu.Lock()
	link := s.link
	c.mu.Unlock()
	if link == nil {
		return
	}
	err := link.Send(kindStatus, status{Muted: link.Muted(), CameraOff: !link.Sending()})
	if err != nil && !errors.Is(err, rtc.ErrNotOpen) {
		slog.Debug("call status not sent", "err", err)
	}
}

func (c *Calls) tellSize(s *session) {
	c.mu.Lock()
	link, own := s.link, s.own
	c.mu.Unlock()
	if link == nil || own.Width == 0 {
		return
	}
	err := link.Send(kindSize, size{Width: own.Width, Height: own.Height})
	if err != nil && !errors.Is(err, rtc.ErrNotOpen) {
		slog.Debug("call size not sent", "err", err)
	}
}

func (c *Calls) ownSized(s *session, sz livecam.Size) {
	c.mu.Lock()
	if c.now != s {
		c.mu.Unlock()
		return
	}
	changed := s.own != sz
	s.own = sz
	pics := s.pics
	c.mu.Unlock()
	if !changed {
		return
	}
	slog.Info("call camera size", "width", sz.Width, "height", sz.Height)
	if pics != nil {
		pics.self.resize(sz)
		pics.replace()
	}
	c.tellSize(s)
	c.changed(s)
}

func (c *Calls) received(s *session) func(rtc.Message) {
	return func(m rtc.Message) {
		switch m.Kind {
		case kindStatus:
			var st status
			if err := json.Unmarshal(m.Data, &st); err != nil {
				return
			}
			c.mu.Lock()
			if c.now != s {
				c.mu.Unlock()
				return
			}
			s.RemoteMuted, s.RemoteCameraOff = st.Muted, st.CameraOff
			pics := s.pics
			c.mu.Unlock()
			pics.showRemote(!st.CameraOff)
			c.changed(s)
		case kindSize:
			var sz size
			if err := json.Unmarshal(m.Data, &sz); err != nil || sz.Width <= 0 || sz.Height <= 0 {
				return
			}
			c.mu.Lock()
			if c.now != s {
				c.mu.Unlock()
				return
			}
			s.remote = livecam.Size{Width: sz.Width, Height: sz.Height}
			pics := s.pics
			c.mu.Unlock()
			slog.Info("call remote size", "width", sz.Width, "height", sz.Height)
			if pics != nil {
				pics.remote.resize(livecam.Size{Width: sz.Width, Height: sz.Height})
				pics.replace()
			}
			c.changed(s)
		default:
			slog.Debug("call message ignored", "kind", m.Kind)
		}
	}
}
