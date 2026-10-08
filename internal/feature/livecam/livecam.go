package livecam

import (
	"fmt"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/camera/helper"
	"github.com/ygelfand/libcountertop/pkg/camera/live"
	"github.com/ygelfand/libcountertop/pkg/camera/session"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/mtkcamera"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(36))
}

type Camera struct{ *live.Camera }

func (c Camera) Restore(config.Config) { c.Publish() }

const (
	mounted       = 3
	helperService = "lanovo_camera"
)

var (
	qualcommMain = []string{"1600x1200", "1280x960", "1024x768", "800x600", "640x480"}
	mediatekMain = []string{"1280x720", "864x480", "640x480"}
	subWidths    = []int{960, 768, 640, 480, 320}
)

var Get = sync.OnceValue(func() Camera {
	b := board.Current()
	o := live.Options{
		Vendor: qualcomm,
		Streams: live.Streams{
			Main:        qualcommMain,
			MainDefault: fmt.Sprintf("%dx%d", b.CameraWidth, b.CameraHeight),
			Sub:         subWidths,
			SubDefault:  b.SubWidth,
		},
		Mirror: b.CameraMirror,
		Turn:   func() int { return turnFor(int(display.Get().Orientation())) },
		Open:   func(c helper.Config) (session.Transport, error) { return mtkcamera.Open(c) },
		Muted:  func() bool { return b.MicMutesCamera && privacy.Get().MicMuted() },
		Restart: func() error {
			return prop.Restart(prop.Local, helperService)
		},
		Saved:  func() schema.Camera { return config.Get().Camera },
		Writer: func() schema.CameraWriter { return config.Set().Camera() },
		Device: component.DeviceCamera,
	}
	if b.SoC == board.MediaTek {
		o.Vendor = mediatek
		o.Streams.Main = mediatekMain
		o.Turn = func() int { return 0 }
	}
	return Camera{live.New(o)}
})

func turnFor(device int) int {
	return (mounted - ((device/90)%4+4)%4 + 8) % 4
}

var qualcomm = live.Vendor{
	EVMin: -12,
	EVMax: 12,
	Scenes: []string{
		"auto",
		"landscape",
		"snow",
		"beach",
		"sunset",
		"night",
		"portrait",
		"sports",
		"steadyphoto",
		"candlelight",
		"fireworks",
		"party",
		"night-portrait",
		"theatre",
		"action",
		"hdr",
	},
	Balances: []string{
		"auto",
		"incandescent",
		"fluorescent",
		"warm-fluorescent",
		"daylight",
		"cloudy-daylight",
		"twilight",
		"shade",
	},
	Effects: []string{
		"none",
		"mono",
		"negative",
		"solarize",
		"sepia",
		"posterize",
		"whiteboard",
		"blackboard",
		"aqua",
	},
	Sliders: true,
	Noises:  []string{"off", "fast", "high_quality", "minimal"},
	Rates:   []int{15, 20, 24, 30},
}

var mediatek = live.Vendor{
	EVMin: -1,
	EVMax: 1,
	Scenes: []string{
		"auto",
		"portrait",
		"landscape",
		"night",
		"night-portrait",
		"theatre",
		"beach",
		"snow",
		"sunset",
		"steadyphoto",
		"sports",
		"party",
		"candlelight",
	},
	Balances: []string{
		"auto",
		"incandescent",
		"fluorescent",
		"daylight",
		"cloudy-daylight",
		"twilight",
		"shade",
	},
	Effects: []string{"none", "mono", "negative", "sepia", "aqua", "whiteboard", "blackboard"},
	ISOs:    []string{"auto", "100", "200", "400", "800", "1600"},
	Levels:  true,
	Rates:   []int{15, 20, 30},
}
