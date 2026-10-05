// Package api presents the display to Home Assistant over the ESPHome native API.
//
// It owns no entities. What it serves is whatever the components registered, collected at
// start-up: their entities, and the handlers that answer without one.
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
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/bluetooth"
	"github.com/ygelfand/LANovo/internal/feature/voice"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/safe"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	// Last: it serves the registry, so nothing should still be coming up when it starts listening.
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

// Start builds the server. Not the constructor, because what the server serves is the registry,
// and the registry is only complete once every package's init has run.
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
			BluetoothFeatures: bluetooth.Get().Advertise(),
		},
		PSK:    psk,
		Logger: slog.Default(),

		// Persist a key Home Assistant pushes, or the next connection reverts to the old one.
		OnSetEncryptionKey: func(k esphome.PSK) error { return writePSK(layout.KeyPath, k) },

		OnSubscribed: func() {
			adopted()
			component.Subscribed.Emit(struct{}{})
		},

		Handler: esphome.Chain(append([]esphome.Handler{ents}, component.Default().Handlers()...)...),
	}
	return nil
}

// subDevices groups entities onto pages of their own, since Home Assistant puts every entity a
// device has on one page and there are more here than anyone wants to read.
//
// Which pages there are is component's to say. This puts the device's name in front of each,
// because Home Assistant shows what it is given verbatim: a bare "Playback" is unreadable in a
// house with several of these.
func subDevices(name string) []esphome.Device {
	pages := component.SubDevices()

	out := make([]esphome.Device, 0, len(pages))
	for _, p := range pages {
		out = append(out, esphome.Device{ID: p.ID, Name: name + " " + p.Name})
	}
	return out
}

func (a *API) Run(ctx context.Context) error {
	for {
		ln, err := net.Listen("tcp", a.srv.Addr)
		if err != nil {
			return fmt.Errorf("api: listen %s: %w", a.srv.Addr, err)
		}
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

		if err != nil || ctx.Err() != nil {
			return err
		}
	}
}

// fire puts an event on Home Assistant's bus. Nothing happens before the server is up or while no
// client is subscribed: an event nobody is listening for is not a failure, and the component that
// asked for it has nothing useful to do about one.
func (a *API) fire(e component.Event) {
	if a.srv == nil {
		return
	}
	if err := a.srv.FireEvent(e.Name, e.Data); err != nil {
		slog.Debug("firing an event failed", "event", e.Name, "err", err)
	}
}

// Reconnect drops every client and serves afresh, which is how a change to what the device says it
// is reaches Home Assistant.
func (a *API) Reconnect() {
	select {
	case a.reconnect <- struct{}{}:
	default:
	}
}

// loadPSK reads the device's key, and answers the reserved zero key when there is none.
//
// Zero is what unprovisioned means: the transport is still Noise, any client may connect, and
// Home Assistant then pushes a real key that OnSetEncryptionKey keeps. Nothing has to carry a
// secret off the device, which is what let the onboarding page go.
//
// Only a missing file is a new device. A key that cannot be read is not grounds for falling back
// to zero — that would hand an adopted device to anyone on the network.
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

// adopted records that Home Assistant has the device, which is what takes the onboarding screen
// away. Written once: it is the same answer every subscription after the first.
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
