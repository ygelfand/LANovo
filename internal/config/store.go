package config

import (
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"github.com/ygelfand/libcountertop/pkg/settings/storage"
	"maps"
	"slices"
	"sync"
)

var once sync.Once
var shared *Store
var loadErr error

func store() *Store    { once.Do(func() { shared, loadErr = Load(layout.StatePath) }); return shared }
func Get() Config      { return store().Get() }
func Set() Writer      { return store().Set() }
func LoadError() error { store(); return loadErr }
func Started(d Device) { store().started(d) }
func Use(path string)  { once.Do(func() {}); shared, loadErr = Load(path) }

type Store struct{ *storage.Store[Config] }

func Load(path string) (*Store, error) {
	s, err := storage.Load(path, Defaults(), cloneConfig, func(data []byte, c *Config) error { return schema.MigrateNetwork(data, &c.Network) })
	return &Store{s}, err
}
func (s *Store) Set() Writer      { return Writer{st: s} }
func (s *Store) started(d Device) { s.Runtime(func(c *Config) { c.Device = d }) }
func cloneConfig(c Config) Config {
	c.Wake.Words = slices.Clone(c.Wake.Words)
	c.Cast.YouTube.Skip = slices.Clone(c.Cast.YouTube.Skip)
	c.Camera.Settings = maps.Clone(c.Camera.Settings)
	c.Home.Control = maps.Clone(c.Home.Control)
	c.Home.Group = maps.Clone(c.Home.Group)
	c.Home.Picks = maps.Clone(c.Home.Picks)
	for key, p := range c.Home.Picks {
		c.Home.Picks[key] = p.Clone()
	}
	return c
}
