package rtsp

import shared "github.com/ygelfand/libcountertop/pkg/media/rtsp"

type Server = shared.Server
type Stream = shared.Stream

var NewStream = shared.NewStream
var NALUs = shared.NALUs

func NewServer() *Server {
	return shared.NewServerWithIdentity(shared.Identity{Name: "LANovo", CNAMEPrefix: "lanovo-"})
}
