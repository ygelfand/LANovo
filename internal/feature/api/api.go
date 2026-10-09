package api

import (
	"errors"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/homeassistant/nativeapi"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/bluetooth"
	"github.com/ygelfand/LANovo/internal/feature/voice"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/logging"
)

func init() {
	component.Register(sharedcomponent.Network, Get, sharedcomponent.Order(99),
		sharedcomponent.Supervise(service.Restart(2*time.Second, time.Minute)))
}

var get = sync.OnceValue(func() *nativeapi.Server {
	s := nativeapi.New(nativeapi.Dependencies{
		KeyPath:      layout.KeyPath,
		Identity:     identity,
		Bluetooth:    func() esphome.BluetoothFeature { return bluetooth.Get().Features() },
		Registry:     component.Default(),
		Adoption:     adoption{},
		OnSubscribed: func() { component.Subscribed.Emit(struct{}{}) },
		Logs:         logging.Log.Stream,
	})
	component.Reconnect.Listen(func(struct{}) { s.Reconnect() })
	component.Fire.Listen(s.Fire)
	component.Settings.Add(s.Controls())
	return s
})

func Get() *nativeapi.Server { return get() }

func identity() (nativeapi.Identity, error) {
	mac := wifi.Get().MAC()
	if mac == "" {
		return nativeapi.Identity{}, errors.New("api: the radio has no address yet")
	}

	device := config.Get().Device
	var devices []esphome.Device
	for _, p := range component.SubDevices() {
		devices = append(devices, esphome.Device{ID: p.ID, Name: device.Name + " " + p.Name})
	}

	return nativeapi.Identity{
		Addr:     device.Addr,
		Platform: layout.Platform,
		Board:    layout.Board,
		Info: esphome.Info{
			Name:          layout.Slug(device.Name),
			FriendlyName:  device.Name,
			MACAddress:    mac,
			Manufacturer:  layout.Manufacturer,
			Model:         device.Model,
			Version:       layout.Version,
			ProjectName:   layout.Manufacturer + "." + board.Current().Name,
			Devices:       devices,
			VoiceFeatures: voice.Features,
		},
	}, nil
}

type adoption struct{}

func (adoption) Adopted() bool            { return config.Get().API.Adopted }
func (adoption) SetAdopted(on bool) error { return config.Set().API().Adopted(on) }
