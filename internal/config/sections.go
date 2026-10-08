package config

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"github.com/ygelfand/libcountertop/pkg/settings/storage"
)

func section[T any](field func(*Config) *T) storage.Binding[Config, T] {
	return storage.Bind(func() *storage.Store[Config] { return store().Store }, field)
}

func apiOf(c *Config) *schema.API             { return &c.API }
func bluetoothOf(c *Config) *schema.Bluetooth { return &c.Bluetooth }
func callOf(c *Config) *schema.Call           { return &c.Call }
func cameraOf(c *Config) *schema.Camera       { return &c.Camera }
func castOf(c *Config) *schema.Cast           { return &c.Cast }
func clockOf(c *Config) *schema.Clock         { return &c.Clock }
func feedbackOf(c *Config) *schema.Feedback   { return &c.Feedback }
func homeOf(c *Config) *schema.Home           { return &c.Home }
func idleOf(c *Config) *schema.Idle           { return &c.Idle }
func mediaOf(c *Config) *schema.Media         { return &c.Media }
func networkOf(c *Config) *schema.Network     { return &c.Network }
func posterOf(c *Config) *schema.Poster       { return &c.Poster }
func rtspOf(c *Config) *schema.RTSP           { return &c.RTSP }
func screenOf(c *Config) *schema.Screen       { return &c.Screen }
func sendspinOf(c *Config) *schema.Sendspin   { return &c.Sendspin }
func timeOf(c *Config) *schema.Time           { return &c.Time }
func updateOf(c *Config) *schema.Update       { return &c.Update }
func visualOf(c *Config) *schema.Visual       { return &c.Visual }
func wakeOf(c *Config) *schema.Wake           { return &c.Wake }
func weatherOf(c *Config) *schema.Weather     { return &c.Weather }

var (
	FeedbackSection  = section(feedbackOf)
	APISection       = section(apiOf)
	NetworkSection   = section(networkOf)
	CallSection      = section(callOf)
	CastSection      = section(castOf)
	HomeSection      = section(homeOf)
	PosterSection    = section(posterOf)
	SendspinSection  = section(sendspinOf)
	BluetoothSection = section(bluetoothOf)
	MediaSection     = section(mediaOf)
	WakeSection      = section(wakeOf)
	ScreenSection    = section(screenOf)
	ClockSection     = section(clockOf)
	IdleSection      = section(idleOf)
	VisualSection    = section(visualOf)
	WeatherSection   = section(weatherOf)
)
