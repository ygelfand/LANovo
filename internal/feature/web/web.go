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

	"github.com/ygelfand/libcountertop/pkg/hook"
	netaddress "github.com/ygelfand/libcountertop/pkg/network/address"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() {
	component.Register(component.Network, Get, component.Order(50))
}

const Port = 80

const shutdown = 2 * time.Second

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

func Adopted() bool { return config.Get().API.Adopted }

func address() string {
	for _, ip := range netaddress.Addresses() {
		if ip.To4() != nil {
			return ip.String()
		}
	}
	return ""
}
