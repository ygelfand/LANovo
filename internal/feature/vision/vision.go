package vision

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
)

func init() {
	component.Register(component.Device, Get, component.Order(40))
}

const Wait = 5 * time.Second

type Vision struct {
	entity *esphome.Camera

	mu sync.Mutex
}

var (
	once   sync.Once
	shared *Vision
)

func Get() *Vision {
	once.Do(func() {
		shared = &Vision{}
		shared.build()
	})
	return shared
}

func (v *Vision) Name() string { return "vision" }

func (v *Vision) Entities() []esphome.Entity { return []esphome.Entity{v.entity} }

func (v *Vision) build() {
	v.entity = &esphome.Camera{
		Base: esphome.Base{
			ObjectID: "camera",
			DeviceID: component.DeviceCamera,
			Name:     "Camera",
			Icon:     "mdi:cctv",
		},
		Image: v.Still,
	}
}

var ErrCovered = errors.New("vision: the camera is covered")

func (v *Vision) Still() ([]byte, error) {
	if privacy.Get().CameraCovered() {
		return nil, ErrCovered
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	pic, err := livecam.Still(Wait)
	if err != nil {
		return nil, err
	}
	return encode(pic)
}

func encode(pic livecam.Picture) ([]byte, error) {
	w, h := pic.Width, pic.Height
	if w <= 0 || h <= 0 || len(pic.RGBA) < w*h*4 {
		return nil, errors.New("vision: a short picture")
	}
	img := &image.RGBA{Pix: pic.RGBA, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: Quality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

const Quality = 85
