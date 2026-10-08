package rtspd

import (
	"log/slog"
	"net"
	"strconv"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/lib/rtsp"
)

const Port = 8554

var paths = [...]string{"main", "sub"}

func init() {
	component.Register(component.Device, Get, component.Order(70))
}

type Server struct {
	sw *esphome.Switch

	mu    sync.Mutex
	srv   *rtsp.Server
	pumps [len(paths)]*pump
	desc  *describer
}

var (
	once   sync.Once
	shared *Server
)

func Get() *Server {
	once.Do(func() {
		shared = &Server{}
		shared.build()
		livecam.StreamsChanged.Listen(func([]int) { shared.reload() })
	})
	return shared
}

func (s *Server) Name() string { return "rtsp" }

func (s *Server) Entities() []esphome.Entity { return []esphome.Entity{s.sw} }

func (s *Server) Restore(cfg config.Config) {
	s.sw.Set(cfg.RTSP.Enabled)
	if cfg.RTSP.Enabled {
		s.start()
	}
}

func (s *Server) build() {
	s.sw = &esphome.Switch{
		Base: esphome.Base{ObjectID: "rtsp", Name: "RTSP server", Icon: "mdi:cctv",
			Category: esphome.CategoryConfig, DeviceID: component.DeviceCamera},
	}
	s.sw.OnCommand = s.SetEnabled
}

func (s *Server) SetEnabled(on bool) {
	s.sw.Set(on)
	if err := config.Set().RTSP().Enabled(on); err != nil {
		slog.Error("saving a setting failed", "setting", s.sw.ObjectID, "err", err)
	}
	if on {
		s.start()
	} else {
		s.stop()
	}
}

func (s *Server) reload() {
	if !config.Get().RTSP.Enabled {
		return
	}
	s.stop()
	s.start()
}

func (s *Server) start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != nil {
		return
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(Port))
	if err != nil {
		slog.Error("the rtsp server could not listen", "port", Port, "err", err)
		return
	}
	srv := rtsp.NewServer()
	for _, i := range served() {
		path := paths[i]
		st := rtsp.NewStream()
		p := &pump{
			at:      i,
			stream:  st,
			allowed: func() bool { return !privacy.Get().CameraCovered() },
		}
		st.OnDemand = p.demand
		s.pumps[i] = p
		srv.Handle(path, st)
	}
	s.srv = srv
	s.desc = describe()
	go func() {
		if err := srv.Serve(ln); err != nil {
			slog.Warn("the rtsp server stopped", "err", err)
		}
	}()
	for _, i := range served() {
		slog.Info("rtsp serving", "url", "rtsp://<device>:"+strconv.Itoa(Port)+"/"+paths[i])
	}
}

func (s *Server) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv == nil {
		return
	}
	_ = s.srv.Close()
	s.srv = nil
	s.desc.close()
	s.desc = nil
	for i, p := range s.pumps {
		if p != nil {
			p.halt()
		}
		s.pumps[i] = nil
	}
}
