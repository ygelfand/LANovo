package config

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"github.com/ygelfand/libcountertop/pkg/settings/storage"

	"github.com/ygelfand/LANovo/internal/layout"
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
	s, err := storage.Load(
		path,
		Defaults(),
		cloneConfig,
		func(data []byte, c *Config) error { return schema.MigrateNetwork(data, &c.Network) },
	)
	return &Store{s}, err
}
func (s *Store) Set() Writer      { return Writer{st: s} }
func (s *Store) started(d Device) { s.Runtime(func(c *Config) { c.Device = d }) }
func cloneConfig(c Config) Config {
	c.Wake = c.Wake.Clone()
	c.Cast = c.Cast.Clone()
	c.Camera = c.Camera.Clone()
	c.Home = c.Home.Clone()
	return c
}
