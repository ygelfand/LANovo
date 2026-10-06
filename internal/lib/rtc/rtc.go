// Package rtc preserves the product's SDP identity on the shared transport.
package rtc

import (
	"github.com/pion/webrtc/v4"
	shared "github.com/ygelfand/libcountertop/pkg/media/rtc"
)

type Audio = shared.Audio
type Frame = shared.Frame
type Camera = shared.Camera
type Screen = shared.Screen
type Message = shared.Message
type Media = shared.Media
type Stats = shared.Stats
type Link = shared.Link

const FrameSamples = shared.FrameSamples
const PlayRate = shared.PlayRate
const PlayChannels = shared.PlayChannels

var ErrNotOpen = shared.ErrNotOpen
var Keyframe = shared.Keyframe

func New(m Media, ended func(webrtc.PeerConnectionState)) (*Link, error) {
	return shared.NewWithStreamID(m, ended, "lanovo")
}
