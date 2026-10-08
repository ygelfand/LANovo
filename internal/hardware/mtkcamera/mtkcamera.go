package mtkcamera

import (
	shared "github.com/ygelfand/libcountertop/pkg/camera/helper"

	"github.com/ygelfand/LANovo/internal/layout"
)

type Config = shared.Config
type Frame = shared.Frame
type Stream = shared.Stream

var Timeout = shared.Timeout

func Open(cfg Config) (*Stream, error) { return shared.Open(layout.CameraSocket, cfg) }
