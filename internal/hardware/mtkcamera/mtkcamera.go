package mtkcamera

import (
	"github.com/ygelfand/libcountertop/pkg/camera/camerafeed"

	"github.com/ygelfand/LANovo/internal/layout"
)

type Config = camerafeed.Config
type Frame = camerafeed.Frame
type Stream = camerafeed.Stream

var Timeout = camerafeed.Timeout

func Open(cfg Config) (*Stream, error) { return camerafeed.Open(layout.CameraSocket, cfg) }
