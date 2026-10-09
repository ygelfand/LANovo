package network

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/fetch"
	"github.com/ygelfand/libcountertop/pkg/network/policy"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
)

func init() {
	fetch.UseAsDefault()
	component.Register(sharedcomponent.Network, Get, sharedcomponent.Order(5))
}

type Network struct{ *policy.Policy }

var get = sync.OnceValue(func() *Network {
	n := &Network{policy.New(config.NetworkSection)}
	component.Settings.Add(n.Settings())
	return n
})

func Get() *Network                          { return get() }
func (n *Network) Restore(cfg config.Config) { n.Policy.Restore(cfg.Network.Verify) }
