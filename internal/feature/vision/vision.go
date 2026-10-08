package vision

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/camera/vision"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(40))
}

var Get = sync.OnceValue(func() *vision.Vision {
	return vision.New(vision.Options{
		DeviceID: component.DeviceCamera,
		Covered:  func() bool { return privacy.Get().CameraCovered() },
		Picture:  livecam.Get().Still,
	})
})
