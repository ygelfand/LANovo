// Package surface supplies this product's socket and protocol to libcountertop.
package surface

import (
	"time"

	shared "github.com/ygelfand/libcountertop/pkg/display/surface"
)

const Socket = "/dev/socket/lanovo-surface"

type Subsample = shared.Subsample
type Crypt = shared.Crypt
type PCM = shared.PCM
type Played = shared.Played
type Matrix = shared.Matrix
type Status = shared.Status
type Rect = shared.Rect
type Layer = shared.Layer
type Placement = shared.Placement
type Client = shared.Client
type Glow = shared.Glow
type UILayer = shared.UILayer

const (
	AVC         = shared.AVC
	Broken      = shared.Broken
	CryptCBCS   = shared.CryptCBCS
	CryptCENC   = shared.CryptCENC
	CryptClear  = shared.CryptClear
	FeedFloat   = shared.FeedFloat
	FeedHalf    = shared.FeedHalf
	Full        = shared.Full
	HEVC        = shared.HEVC
	LineFloats  = shared.LineFloats
	Opaque      = shared.Opaque
	PointFloats = shared.PointFloats
	QuadFloats  = shared.QuadFloats
	SampleEnd   = shared.SampleEnd
	SampleKey   = shared.SampleKey
	Secure      = shared.Secure
	SplatHalf   = shared.SplatHalf
	VP8         = shared.VP8
	VP9         = shared.VP9
	WithFeed    = shared.WithFeed
	WithLight   = shared.WithLight
	WithLines   = shared.WithLines
	WithPre     = shared.WithPre
	WithSplat   = shared.WithSplat
)

var (
	ErrVersion = shared.ErrVersion
	MIME       = shared.MIME
	Widevine   = shared.Widevine
)

func Dial(path string) (*Client, error) { return shared.Dial(path) }
func DialWait(path string, wait time.Duration) (*Client, error) {
	return shared.DialWait(path, wait)
}
