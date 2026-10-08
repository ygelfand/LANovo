package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/runtime/safe"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/bluetooth"
	"github.com/ygelfand/LANovo/internal/feature/voice"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Network, Get, component.Order(99),
		component.Supervise(service.Restart(2*time.Second, time.Minute)))
}

// One of Home Assistant's keepalive intervals; it gives up on us at 4.5 of them.
const writeTimeout = 20 * time.Second

type API struct {
	srv  *esphome.Server
	name string

	reconnect chan struct{}
	announced sync.Once
	listening atomic.Bool
}

var (
	once   sync.Once
	shared *API
)

func Get() *API {
	once.Do(func() {
		shared = &API{reconnect: make(chan struct{}, 1)}
		component.Reconnect.Listen(func(struct{}) { shared.Reconnect() })
		component.Fire.Listen(func(e component.Event) { shared.fire(e) })
	})
	return shared
}

func (a *API) Name() string { return "api" }

func (a *API) Start(context.Context) error {
	psk, err := loadPSK(layout.KeyPath)
	if err != nil {
		return err
	}

	mac := wifi.Get().MAC()
	if mac == "" {
		return fmt.Errorf("api: the radio has no address yet")
	}

	device := config.Get().Device
	ents := esphome.NewEntities()
	if err := ents.Add(component.Default().Entities()...); err != nil {
		return err
	}
	if err := ents.AddActions(component.Default().Actions()...); err != nil {
		return err
	}

	a.name = layout.Slug(device.Name)
	a.srv = &esphome.Server{
		Addr:         device.Addr,
		WriteTimeout: writeTimeout,
		Info: esphome.Info{
			Name:         a.name,
			FriendlyName: device.Name,
			MACAddress:   mac,
			Manufacturer: layout.Manufacturer,
			Model:        device.Model,
			Version:      layout.Version,
			ProjectName:  layout.Manufacturer + "." + board.Current().Name,

			Devices: subDevices(device.Name),

			VoiceFeatures:     voice.Features,
			BluetoothFeatures: bluetooth.Get().Features(),
		},
		PSK:    psk,
		Logger: slog.Default(),

		OnSetEncryptionKey: func(k esphome.PSK) error { return writePSK(layout.KeyPath, k) },

		OnSubscribed: func() {
			adopted()
			component.Subscribed.Emit(struct{}{})
		},

		Handler: esphome.Chain(
			append([]esphome.Handler{ents}, component.Default().Handlers()...)...),
	}
	return nil
}

func subDevices(name string) []esphome.Device {
	pages := component.SubDevices()

	out := make([]esphome.Device, 0, len(pages))
	for _, p := range pages {
		out = append(out, esphome.Device{ID: p.ID, Name: name + " " + p.Name})
	}
	return out
}

func (a *API) Startup() component.Progress {
	if !a.listening.Load() {
		return component.Progress{Doing: "opening the port"}
	}
	return component.Progress{Done: true, Doing: fmt.Sprintf("port %d", layout.Port)}
}

func (a *API) Run(ctx context.Context) error {
	defer a.listening.Store(false)
	for {
		ln, err := net.Listen("tcp", a.srv.Addr)
		if err != nil {
			return fmt.Errorf("api: listen %s: %w", a.srv.Addr, err)
		}
		a.listening.Store(true)
		slog.Info("serving", "addr", ln.Addr(), "node", a.name)

		a.announced.Do(func() {
			safe.Go("mdns", func() { a.advertise(ctx, ln.Addr().(*net.TCPAddr).Port) })
		})

		serving, stop := context.WithCancel(ctx)
		go func() {
			select {
			case <-a.reconnect:
			case <-serving.Done():
			}
			stop()
		}()

		err = a.srv.Serve(serving, ln)
		stop()
		a.listening.Store(false)

		if err != nil || ctx.Err() != nil {
			return err
		}
	}
}

func (a *API) fire(e component.Event) {
	if a.srv == nil {
		return
	}
	if err := a.srv.FireEvent(e.Name, e.Data); err != nil {
		slog.Debug("firing an event failed", "event", e.Name, "err", err)
	}
}

func (a *API) Reconnect() {
	select {
	case a.reconnect <- struct{}{}:
	default:
	}
}

// ESPHome reserves the all-zero key for an unprovisioned device.
func loadPSK(path string) (*esphome.PSK, error) {
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		k, err := esphome.ParsePSK(strings.TrimSpace(string(b)))
		if err != nil {
			return nil, fmt.Errorf("api: key at %s: %w", path, err)
		}
		return &k, nil

	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("api: key at %s: %w", path, err)
	}

	slog.Info("unprovisioned: the api takes any client until Home Assistant sets a key")
	return esphome.Unprovisioned(), nil
}

func adopted() {
	if config.Get().API.Adopted {
		return
	}

	if err := config.Set().API().Adopted(true); err != nil {
		slog.Error("recording adoption failed", "err", err)
		return
	}
	slog.Info("adopted")
}

func (a *API) SetAdopted(on bool) {
	if err := config.Set().API().Adopted(on); err != nil {
		slog.Error("saving a setting failed", "setting", "api.adopted", "err", err)
	}
}

func writePSK(path string, k esphome.PSK) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(k.String()+"\n"), 0o600)
}
