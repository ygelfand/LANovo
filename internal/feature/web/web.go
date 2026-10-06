// Package web serves what the device says about itself, and shows the code that adds it.
//
// Setup does not come through here. The panel's code opens Home Assistant's own add flow and the
// device answers on the reserved zero key until Home Assistant sets one, so nothing has to carry
// a secret from the device to the person setting it up. What is left on port 80 is the status
// page, which holds no secrets and is unauthenticated on purpose.
package web

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/metrics"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/hook"
)

func init() {
	component.Register(component.Network, Get, component.Order(50))
}

// Port is where the pages are served. Eighty, because the address is typed off a screen by
// someone standing in front of it and a port number is one more thing to get wrong.
const Port = 80

// shutdown is how long the server is given to finish what it is answering.
const shutdown = 2 * time.Second

// watch is how often the onboarding screen reconsiders itself, for an address that arrives after
// the panel is already up.
const watch = 5 * time.Second

const AdoptURL = "https://my.home-assistant.io/redirect/config_flow_start/?domain=esphome"

type Server struct {
	srv *http.Server

	mu    sync.Mutex
	shown string

	Offered hook.Hook[string]
}

func (s *Server) Offering() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shown
}

var (
	once   sync.Once
	shared *Server
)

func Get() *Server {
	once.Do(func() { shared = &Server{} })
	return shared
}

func (s *Server) Name() string { return "web" }

var (
	routesMu sync.Mutex
	routes   = map[string]http.Handler{}
)

func Handle(pattern string, h http.Handler) {
	routesMu.Lock()
	routes[pattern] = h
	routesMu.Unlock()
}

// Run serves until ctx is canceled, and holds the onboarding screen until the device has been
// adopted.
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.status)
	routesMu.Lock()
	for pattern, h := range routes {
		mux.Handle(pattern, h)
	}
	routesMu.Unlock()

	s.srv = &http.Server{
		Addr:              ":" + strconv.Itoa(Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	done := make(chan error, 1)
	go func() {
		slog.Info("serving pages", "port", Port)

		err := s.srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		done <- err
	}()

	t := time.NewTicker(watch)
	defer t.Stop()

	for {
		s.offer()

		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			s.hide()

			stop, cancel := context.WithTimeout(context.Background(), shutdown)
			defer cancel()
			return s.srv.Shutdown(stop)
		case <-t.C:
		}
	}
}

// offer shows the code to scan while the device has not been adopted, and takes it away once it
// has.
func (s *Server) offer() {
	if Adopted() {
		s.hide()
		return
	}

	ip := address()
	if ip == "" {
		return
	}
	addr := net.JoinHostPort(ip, strconv.Itoa(layout.Port))

	s.mu.Lock()
	if s.shown == addr {
		s.mu.Unlock()
		return
	}
	s.shown = addr
	s.mu.Unlock()
	slog.Info("waiting to be adopted", "at", addr)
	s.Offered.Emit(addr)
}

func (s *Server) hide() {
	s.mu.Lock()
	if s.shown == "" {
		s.mu.Unlock()
		return
	}
	s.shown = ""
	s.mu.Unlock()
	s.Offered.Emit("")
}

// Adopted reports whether Home Assistant has taken the device, which is whether it has ever
// subscribed. Not whether the device holds a key: one it was installed with says nothing about
// anyone having added it.
func Adopted() bool { return config.Get().API.Adopted }

// address is where the device can be reached, empty until it has one.
//
// IPv4 only. This ends up in a URL on a screen, to be scanned or typed, and the device's v6
// address would need bracketing there and is not something anyone copies off a panel by hand.
// The lease this device runs on is v4, so holding out for one costs nothing.
func address() string {
	for _, ip := range metrics.Addresses() {
		if ip.To4() != nil {
			return ip.String()
		}
	}
	return ""
}
