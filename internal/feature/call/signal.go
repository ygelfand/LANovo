package call

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/libcountertop/pkg/fetch"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

const postWait = 5 * time.Second

type envelope struct {
	Call   string   `json:"call"`
	From   string   `json:"from,omitempty"`
	SDP    string   `json:"sdp,omitempty"`
	Video  bool     `json:"video,omitempty"`
	Width  int      `json:"width,omitempty"`
	Height int      `json:"height,omitempty"`
	Reason Reason   `json:"reason,omitempty"`
	Name   string   `json:"name,omitempty"`
	Model  string   `json:"model,omitempty"`
	Board  string   `json:"board,omitempty"`
	Caps   []string `json:"caps,omitempty"`
}

func (c *Calls) offered(w http.ResponseWriter, r *http.Request) {
	var m envelope
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil || m.Call == "" || m.SDP == "" {
		http.Error(w, "bad offer", http.StatusBadRequest)
		return
	}
	cfg := config.Get().Call
	if !cfg.Incoming {
		http.Error(w, "not taking calls", http.StatusForbidden)
		return
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "bad address", http.StatusBadRequest)
		return
	}
	p, ok := discovery.Get().Find(m.From)
	if !ok {
		p = discovery.Peer{ID: m.From, Name: m.Name, Model: m.Model, Board: m.Board, Caps: m.Caps, Addrs: []net.IP{net.ParseIP(host)}, Port: web.Port, Seen: time.Now()}
		if p.Name == "" {
			p.Name = host
		}
	}
	c.mu.Lock()
	if c.now != nil {
		c.mu.Unlock()
		http.Error(w, "busy", http.StatusConflict)
		return
	}
	s := c.open(Call{ID: m.Call, Peer: p, State: Ringing, Since: time.Now(), Video: m.Video, CameraOff: m.Video && cfg.AutoAnswer && !cfg.AutoVideo}, net.JoinHostPort(host, strconv.Itoa(web.Port)))
	s.offer = m.SDP
	s.remote = livecam.Size{Width: m.Width, Height: m.Height}
	ringing, quiet := context.WithCancel(s.ctx)
	s.quiet = quiet
	c.mu.Unlock()
	slog.Info("incoming call", "from", p.Name, "call", s.ID, "video", m.Video, "auto", cfg.AutoAnswer)
	c.changed(s)
	w.WriteHeader(http.StatusAccepted)
	if cfg.AutoAnswer {
		c.Answer()
		return
	}
	safe.Go("call ring", func() {
		if ring(ringing) {
			slog.Info("call missed", "from", p.Name)
			c.close(s, ReasonUnanswered)
		}
	})
}

func (c *Calls) answered(w http.ResponseWriter, r *http.Request) {
	var m envelope
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil || m.SDP == "" {
		http.Error(w, "bad answer", http.StatusBadRequest)
		return
	}
	c.mu.Lock()
	s := c.now
	ok := s != nil && s.ID == m.Call && s.State == Calling && s.link != nil
	var pics *pictures
	if ok {
		s.remote = livecam.Size{Width: m.Width, Height: m.Height}
		pics = s.pics
	}
	c.mu.Unlock()
	if !ok {
		http.Error(w, "no such call", http.StatusNotFound)
		return
	}
	if pics != nil {
		pics.remote.resize(livecam.Size{Width: m.Width, Height: m.Height})
		pics.replace()
	}
	if err := s.link.Answered(m.SDP); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		c.fail(s, "answered", err)
		return
	}
	c.talking(s)
}

func (c *Calls) ended(w http.ResponseWriter, r *http.Request) {
	var m envelope
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "bad end", http.StatusBadRequest)
		return
	}
	c.mu.Lock()
	s := c.now
	c.mu.Unlock()
	if s == nil || s.ID != m.Call {
		return
	}
	reason := m.Reason
	switch {
	case reason == ReasonCancelled || reason == ReasonUnanswered:
		reason = ReasonHangup
	case reason == "":
		reason = ReasonHangup
	}
	slog.Info("call ended by peer", "peer", s.Peer.Name, "reason", m.Reason)
	c.close(s, reason)
}

func route(p discovery.Peer) (string, error) {
	for _, a := range p.Addrs {
		if a.To4() != nil {
			return net.JoinHostPort(a.String(), strconv.Itoa(p.Port)), nil
		}
	}
	return "", ErrNoRoute
}

func post(ctx context.Context, addr, what string, m envelope) (int, error) {
	body, err := json.Marshal(m)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+addr+"/call/"+what, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := fetch.Client(postWait).Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}
