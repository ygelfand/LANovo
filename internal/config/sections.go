package config

import "github.com/ygelfand/libcountertop/pkg/settings/storage"

func section[T any](field func(*Config) *T) storage.Binding[Config, T] {
	return storage.Bind(func() *storage.Store[Config] { return store().Store }, field)
}

// Saved sections resolve the current store lazily and preserve its atomic update policy.
var (
	FeedbackSection  = section(func(c *Config) *Feedback { return &c.Feedback })
	APISection       = section(func(c *Config) *API { return &c.API })
	NetworkSection   = section(func(c *Config) *Network { return &c.Network })
	CallSection      = section(func(c *Config) *Call { return &c.Call })
	CastSection      = section(func(c *Config) *Cast { return &c.Cast })
	HomeSection      = section(func(c *Config) *Home { return &c.Home })
	PosterSection    = section(func(c *Config) *Poster { return &c.Poster })
	SendspinSection  = section(func(c *Config) *Sendspin { return &c.Sendspin })
	BluetoothSection = section(func(c *Config) *Bluetooth { return &c.Bluetooth })
	MediaSection     = section(func(c *Config) *Media { return &c.Media })
	WakeSection      = section(func(c *Config) *Wake { return &c.Wake })
	ScreenSection    = section(func(c *Config) *Screen { return &c.Screen })
	ClockSection     = section(func(c *Config) *Clock { return &c.Clock })
	IdleSection      = section(func(c *Config) *Idle { return &c.Idle })
	VisualSection    = section(func(c *Config) *Visual { return &c.Visual })
	WeatherSection   = section(func(c *Config) *Weather { return &c.Weather })
)
