// Package mtkcamera selects the product camera socket for the shared helper client.
package mtkcamera

import (
	"github.com/ygelfand/LANovo/internal/layout"
	shared "github.com/ygelfand/libcountertop/pkg/camera/helper"
)

type Config = shared.Config
type Frame = shared.Frame
type Stream = shared.Stream

var Timeout = shared.Timeout

func Open(cfg Config) (*Stream, error) { return shared.Open(layout.CameraSocket, cfg) }
