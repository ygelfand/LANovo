// Package network is how the device treats the rest of the world, which today is one question:
// whether a certificate has to check out before it will download over it.
package network

import (
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/libcountertop/pkg/fetch"
)

func init() {
	fetch.UseAsDefault()
	// Before anything downloads, so nothing goes out under a policy that is about to change.
	component.Register(component.Network, Get, component.Order(5))
}

// Network is the setting and the entity that carries it.
type Network struct{ verify *esphome.Switch }

var (
	once   sync.Once
	shared *Network
)

func Get() *Network {
	once.Do(func() {
		shared = &Network{}
		shared.build()
	})
	return shared
}

func (n *Network) Name() string { return "network" }

func (n *Network) Entities() []esphome.Entity { return []esphome.Entity{n.verify} }

func (n *Network) Restore(cfg config.Config) { n.apply(cfg.Network.Verify) }

// SetVerify turns certificate checking on or off and remembers it. Everything that changes it comes
// through here, so Home Assistant is told whatever asked for it.
func (n *Network) SetVerify(on bool) {
	n.apply(on)

	if err := config.Set().Network().Verify(on); err != nil {
		slog.Error("saving a setting failed", "setting", n.verify.ObjectID, "err", err)
	}
}

// Verifying reports whether certificates are being checked, for anything that wants to say so
// where it can be seen.
func (n *Network) Verifying() bool { return !fetch.Skipping() }

// apply drives the client and the entity, and says so in the log when it is off. Loudly, and every
// time: a device downloading over connections it cannot vouch for should leave a trail.
func (n *Network) apply(on bool) {
	fetch.Verify(on)
	n.verify.Set(on)

	if !on {
		slog.Warn("certificates are not being verified: downloads will be taken on trust")
	}
}

func (n *Network) build() {
	n.verify = &esphome.Switch{
		Base: esphome.Base{
			ObjectID: "verify_certificates",
			Name:     "Verify certificates",
			Icon:     "mdi:certificate",
			Category: esphome.CategoryConfig,
		},
	}

	n.verify.OnCommand = n.SetVerify
}
