// Package a2dp is the device as a Bluetooth speaker.
//
// The pieces all existed and none of them were joined: internal/lib/bt decides what a phone's bytes
// mean, internal/lib/bt/pair decides a pairing, internal/lib/bt/sbc turns frames into samples, and
// internal/hardware/ble carries the line. This is the wiring, and nothing else — no protocol lives
// here, so a fault is in one of the parts rather than spread between them.
package a2dp

import (
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/LANovo/internal/hardware/ble"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/service"
	sharedspeaker "github.com/ygelfand/libcountertop/pkg/bluetooth/a2dp"
	"github.com/ygelfand/libcountertop/pkg/media/pcm"
)

type Sink = sharedspeaker.Sink

var once sync.Once
var shared *Sink

func build() *Sink {
	a := speaker.Sound().Backgrounds()
	return sharedspeaker.New(
		sharedspeaker.Options{
			Queue:            speaker.Get(),
			Radio:            ble.Get,
			Bonds:            layout.StateDir + "/bonds",
			CaptureDirectory: "/data/local/tmp",
			Name:             func() string { return config.Get().Device.Name },
			Enabled:          func() bool { return config.Get().Bluetooth.Speaker },
			SaveEnabled:      func(v bool) error { return config.Set().Bluetooth().Speaker(v) },
			VolumeSet:        func(v int) { volume.Get().Set(config.StreamMedia, v) },
			VolumeChanged: func(f func(int)) func() {
				return volume.Get().Changed.Listen(func(c volume.Change) {
					if c.Stream == config.StreamMedia {
						f(c.Level)
					}
				})
			},
			Media:  func() sharedspeaker.Media { return media.Get() },
			Took:   func(p pcm.Producer) { a.Took(p) },
			Gave:   func(p pcm.Producer) { a.Gave(p) },
			DuckDB: func() float64 { return config.Get().Media.DuckDB },
		},
	)
}
func Get() *Sink { once.Do(func() { shared = build() }); return shared }
func init() {
	component.Register(
		component.Device,
		Get,
		component.Order(70),
		component.Supervise(service.Restart(5*time.Second, time.Minute)),
	)
}

var Trace = sharedspeaker.Trace

func Dump(d time.Duration) (string, error) { return Get().Dump(d) }
