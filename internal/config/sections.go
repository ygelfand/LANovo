package config

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"github.com/ygelfand/libcountertop/pkg/settings/storage"
)

func sharedOf(c *Config) *schema.Shared { return &c.Shared }

var sections = schema.NewSections(func() *storage.Store[Config] { return store().Store }, sharedOf)

var (
	AlertsSection    = sections.Alerts
	FeedbackSection  = sections.Feedback
	APISection       = sections.API
	AppsSection      = sections.Apps
	NetworkSection   = sections.Network
	CallSection      = sections.Call
	CastSection      = sections.Cast
	HomeSection      = sections.Home
	PosterSection    = sections.Poster
	PhotosSection    = sections.Photos
	SendspinSection  = sections.Sendspin
	BluetoothSection = sections.Bluetooth
	MediaSection     = sections.Media
	WakeSection      = sections.Wake
	ScreenSection    = sections.Screen
	ClockSection     = sections.Clock
	IdleSection      = sections.Idle
	VisualSection    = sections.Visual
	WeatherSection   = sections.Weather
	TodoSection      = sections.Todo
	CalendarSection  = sections.Calendar
	RadioSection     = sections.Radio
	VoiceSection     = sections.Voice

	LiftSection = storage.Bind(
		func() *storage.Store[Config] { return store().Store },
		func(c *Config) *int { return &c.Microphone.VisualizerLift },
	)

	MicrophoneSection = storage.Bind(
		func() *storage.Store[Config] { return store().Store },
		func(c *Config) *Microphone { return &c.Microphone },
	)

	AccessSection = storage.Bind(
		func() *storage.Store[Config] { return store().Store },
		func(c *Config) *Access { return &c.Access },
	)
)
