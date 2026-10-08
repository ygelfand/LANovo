package onvif

import shared "github.com/ygelfand/libcountertop/pkg/camera/onvif"

type Probe = shared.Probe
type Profile = shared.Profile
type Device = shared.Device
type Service = shared.Service
type Responder = shared.Responder

const DevicePath = shared.DevicePath
const MediaPath = shared.MediaPath

var ParseProbe = shared.ParseProbe
var NewUUID = shared.NewUUID

func Matches(p Probe, uuid, xaddr, name string) []byte {
	return shared.MatchesHardware(p, uuid, xaddr, name, "LANovo")
}
