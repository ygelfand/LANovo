package drm

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/surface"
	"github.com/ygelfand/libcountertop/pkg/media/widevine"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/dhcp"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(61))
}

var Get = sync.OnceValue(func() *widevine.Provisioner {
	return widevine.New(func() *surface.Client { return display.Get().Helper() }, dhcp.Get())
})
