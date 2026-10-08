package network

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/fetch"
	"github.com/ygelfand/libcountertop/pkg/network/policy"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
)

func init() {
	fetch.UseAsDefault()
	component.Register(component.Network, Get, component.Order(5))
}

type Network struct{ *policy.Policy }

var get = sync.OnceValue(func() *Network { return &Network{policy.New(config.NetworkSection)} })

func Get() *Network                          { return get() }
func (n *Network) Restore(cfg config.Config) { n.Policy.Restore(cfg.Network.Verify) }
