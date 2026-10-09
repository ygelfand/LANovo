package rtspd

import (
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/camera/live"
	"github.com/ygelfand/libcountertop/pkg/camera/onvif"
	camerartspd "github.com/ygelfand/libcountertop/pkg/camera/rtspd"
	"github.com/ygelfand/libcountertop/pkg/media/rtsp"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	setting "github.com/ygelfand/libcountertop/pkg/settings"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/vision"
)

const (
	Port      = 8554
	ONVIFPort = 8000
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(70))
}

type Server struct {
	knobs *setting.Controls[schema.RTSP]
	srv   *camerartspd.Server
}

var (
	once   sync.Once
	shared *Server
)

func Get() *Server {
	once.Do(func() {
		shared = &Server{srv: camerartspd.New(camerartspd.Options{
			Port:     Port,
			Identity: rtsp.Identity{Name: "LANovo", CNAMEPrefix: "lanovo-"},
			Source:   livecam.Get().Sessions(),
			Helper:   "lanovo-camera",
			Served:   livecam.Get().Served,
			FPS:      live.FPS,
			Bitrate:  livecam.Get().Bitrate,
			Allowed:  func() bool { return !privacy.Get().CameraCovered() },
			ONVIF:    describe,
		})}
		shared.build()
		livecam.Get().StreamsChanged.Listen(func([]int) { shared.reload() })
	})
	return shared
}

func describe() onvif.Config {
	dev := config.Get().Device
	serial, _ := prop.Local.Getprop("ro.serialno")
	if serial == "" {
		serial = dev.Name
	}
	return onvif.Config{
		Port: ONVIFPort,
		Device: onvif.Device{
			Manufacturer: "Lenovo",
			Model:        dev.Model,
			Firmware:     "LANovo",
			Serial:       serial,
			Hardware:     dev.Model,
			Name:         dev.Name,
		},
		Hardware: "LANovo",
		Snapshot: vision.Get().Still,
	}
}

func (s *Server) Name() string { return "rtsp" }

func (s *Server) Entities() []esphome.Entity { return s.knobs.Entities() }

func (s *Server) Restore(cfg config.Config) {
	s.knobs.PublishFrom(cfg.RTSP)
	if cfg.RTSP.Enabled {
		s.srv.Start()
	}
}

func (s *Server) build() {
	s.knobs = camerartspd.Controls(
		component.DeviceCamera,
		func() schema.RTSP { return config.Get().RTSP },
		s.enable,
	)
	component.Settings.Add(s.knobs)
}

func (s *Server) SetEnabled(on bool) {
	if err := s.knobs.Change("enabled", setting.OnOff(on)); err != nil {
		slog.Error("saving a setting failed", "setting", "rtsp.enabled", "err", err)
	}
}

func (s *Server) enable(on bool) error {
	err := config.Set().RTSP().Enabled(on)
	if on {
		s.srv.Start()
	} else {
		s.srv.Stop()
	}
	return err
}

func (s *Server) reload() {
	if !config.Get().RTSP.Enabled {
		return
	}
	s.srv.Stop()
	s.srv.Start()
}
