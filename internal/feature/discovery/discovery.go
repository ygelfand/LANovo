package discovery

import (
	"strings"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/media/broadcast"
	shared "github.com/ygelfand/libcountertop/pkg/network/discovery"
	sharedpeer "github.com/ygelfand/libcountertop/pkg/network/peer"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
)

type identity struct{}

func (identity) Self() sharedpeer.Peer {
	b := board.Current()
	return sharedpeer.Peer{
		ID:    ID(),
		Name:  config.Get().Device.Name,
		Model: b.Model,
		Board: b.Name,
		Caps:  caps(),
		Project: strings.ToLower(
			layout.Manufacturer,
		),
		Version: layout.Version,
		Port:    web.Port,
		Secure:  web.SecurePort,
		Host:    layout.Slug(config.Get().Device.Name),
	}
}

func ID() string { return strings.ToLower(strings.ReplaceAll(wifi.Get().MAC(), ":", "")) }
func caps() []string {
	c := []string{"audio", "sync"}
	if board.Current().CameraWidth > 0 {
		c = append(c, "video")
	}
	if config.Get().Call.Broadcasts {
		c = append(c, broadcast.Cap)
	}
	return c
}

var get = sync.OnceValue(func() *shared.Discovery {
	d := shared.New(identity{}, judge{})
	d.Changed.Listen(func(struct{}) { shell.Get().Redraw() })
	return d
})

func Get() *shared.Discovery { return get() }
func init() {
	component.Register(sharedcomponent.Network, Get, sharedcomponent.Order(60))
}
