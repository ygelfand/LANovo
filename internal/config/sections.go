package config

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"github.com/ygelfand/libcountertop/pkg/settings/storage"
)

func sharedOf(c *Config) *schema.Shared { return &c.Shared }

var sections = schema.NewSections(func() *storage.Store[Config] { return store().Store }, sharedOf)

var (
	FeedbackSection  = sections.Feedback
	APISection       = sections.API
	NetworkSection   = sections.Network
	CallSection      = sections.Call
	CastSection      = sections.Cast
	HomeSection      = sections.Home
	PosterSection    = sections.Poster
	SendspinSection  = sections.Sendspin
	BluetoothSection = sections.Bluetooth
	MediaSection     = sections.Media
	WakeSection      = sections.Wake
	ScreenSection    = sections.Screen
	ClockSection     = sections.Clock
	IdleSection      = sections.Idle
	VisualSection    = sections.Visual
	WeatherSection   = sections.Weather
)
