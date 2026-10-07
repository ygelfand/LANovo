package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Clock = schema.Clock
type Delay = schema.Delay
type Position = schema.Position
type Size = schema.Size
type Face = schema.Face

const (
	DelayNever        = schema.DelayNever
	Delay30s          = schema.Delay30s
	Delay1m           = schema.Delay1m
	Delay2m           = schema.Delay2m
	Delay3m           = schema.Delay3m
	Delay5m           = schema.Delay5m
	Delay10m          = schema.Delay10m
	Delay15m          = schema.Delay15m
	PositionTop       = schema.PositionTop
	PositionCenter    = schema.PositionCenter
	PositionBottom    = schema.PositionBottom
	SizeNano          = schema.SizeNano
	SizeMicro         = schema.SizeMicro
	SizeMini          = schema.SizeMini
	SizeSmall         = schema.SizeSmall
	SizeMedium        = schema.SizeMedium
	SizeLarge         = schema.SizeLarge
	FaceNone          = schema.FaceNone
	FacePlain         = schema.FacePlain
	FaceStack         = schema.FaceStack
	FaceCards         = schema.FaceCards
	FaceAnalog        = schema.FaceAnalog
	FaceAnalogSeconds = schema.FaceAnalogSeconds
	FaceSegments      = schema.FaceSegments
	FaceWords         = schema.FaceWords
	FaceOverlap       = schema.FaceOverlap
	DefaultDelay      = schema.DefaultDelay
	DefaultPosition   = schema.DefaultPosition
	DefaultSize       = schema.DefaultSize
	DefaultFace       = schema.DefaultFace
	DefaultDate       = schema.DefaultDate
)

var Delays = schema.Delays
var MediaDelays = schema.MediaDelays
var Positions = schema.Positions
var Sizes = schema.Sizes
var Faces = schema.Faces
var defaultClock = schema.DefaultClock

type ClockWriter struct{ st *Store }

func (w ClockWriter) Face(v Face) error {
	return w.st.Update(func(c *Config) { c.Clock.Face = v })
}

func (w ClockWriter) Date(v bool) error {
	return w.st.Update(func(c *Config) { c.Clock.Date = v })
}

func (w ClockWriter) Position(v Position) error {
	return w.st.Update(func(c *Config) { c.Clock.Position = v })
}

func (w ClockWriter) Size(v Size) error {
	return w.st.Update(func(c *Config) { c.Clock.Size = v })
}

func (w ClockWriter) Ink(v Ink) error {
	return w.st.Update(func(c *Config) { c.Clock.Ink = v })
}
