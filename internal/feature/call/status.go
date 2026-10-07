package call

import (
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/lib/rtc"
	sharedstatus "github.com/ygelfand/libcountertop/pkg/media/callstatus"
)

func (c *Calls) statusController() sharedstatus.Controller[*session] {
	return sharedstatus.Controller[*session]{Options: sharedstatus.Options[*session]{
		Snapshot: func(s *session) (sharedstatus.Link, sharedstatus.Size) {
			c.mu.Lock()
			defer c.mu.Unlock()
			if s.link == nil {
				return nil, sharedstatus.Size{}
			}
			return s.link, sharedstatus.Size(s.own)
		},
		Own: func(s *session, sz sharedstatus.Size) (func(), bool) {
			c.mu.Lock()
			if c.now != s || s.own == livecam.Size(sz) {
				c.mu.Unlock()
				return nil, false
			}
			s.own = livecam.Size(sz)
			pics := s.pics
			c.mu.Unlock()
			return func() {
				if pics != nil {
					pics.self.resize(livecam.Size(sz))
					pics.replace()
				}
			}, true
		},
		Remote: func(s *session, sz sharedstatus.Size) (func(), bool) {
			c.mu.Lock()
			if c.now != s {
				c.mu.Unlock()
				return nil, false
			}
			s.remote = livecam.Size(sz)
			pics := s.pics
			c.mu.Unlock()
			return func() {
				if pics != nil {
					pics.remote.resize(livecam.Size(sz))
					pics.replace()
				}
			}, true
		},
		Status: func(s *session, st sharedstatus.Status) (func(), bool) {
			c.mu.Lock()
			if c.now != s {
				c.mu.Unlock()
				return nil, false
			}
			s.RemoteMuted, s.RemoteCameraOff = st.Muted, st.CameraOff
			pics := s.pics
			c.mu.Unlock()
			return func() {
				if pics != nil {
					pics.showRemote(!st.CameraOff)
				}
			}, true
		},
		Changed: c.changed,
	}}
}
func (c *Calls) tell(s *session)     { c.statusController().Tell(s) }
func (c *Calls) tellSize(s *session) { c.statusController().TellSize(s) }
func (c *Calls) ownSized(s *session, sz livecam.Size) {
	c.statusController().OwnSized(s, sharedstatus.Size(sz))
}
func (c *Calls) received(s *session) func(rtc.Message) { return c.statusController().Received(s) }
