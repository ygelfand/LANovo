package network

import (
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/libcountertop/pkg/fetch"
	"github.com/ygelfand/libcountertop/pkg/network/policy"
	"sync"
)

func init() {
	fetch.UseAsDefault()
	component.Register(component.Network, Get, component.Order(5))
}

type Network struct{ *policy.Policy }

var get = sync.OnceValue(func() *Network {
	return &Network{policy.New(policy.Options{
		Read: func() bool { return config.Get().Network.Verify },
		Save: func(v bool) error { return config.Set().Network().Verify(v) },
	})}
})

func Get() *Network                          { return get() }
func (n *Network) Restore(cfg config.Config) { n.Policy.Restore(cfg.Network.Verify) }
