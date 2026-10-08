package a2dp

import (
	"sync"
	"time"

	sharedvolume "github.com/ygelfand/libcountertop/pkg/audio/volume"
	sharedspeaker "github.com/ygelfand/libcountertop/pkg/bluetooth/a2dp"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/ble"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/service"
)

var once sync.Once
var shared *sharedspeaker.Sink

func build() *sharedspeaker.Sink {
	return sharedspeaker.New(sharedspeaker.Dependencies{
		Queue:            speaker.Get(),
		Radio:            ble.Get,
		Bonds:            layout.StateDir + "/bonds",
		CaptureDirectory: "/data/local/tmp",
		Name:             func() string { return config.Get().Device.Name },
		Settings:         config.BluetoothSection,
		Volume:           sharedvolume.For(volume.Get(), config.StreamMedia),
		Media:            media.Get(),
		Arbitration:      speaker.Sound().Backgrounds(),
		Ducking:          config.MediaSection,
	})
}
func Get() *sharedspeaker.Sink { once.Do(func() { shared = build() }); return shared }
func init() {
	component.Register(
		component.Device,
		Get,
		component.Order(70),
		component.Supervise(service.Restart(5*time.Second, time.Minute)),
	)
}
