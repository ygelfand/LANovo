package gpu

import (
	sharedgpu "github.com/ygelfand/libcountertop/pkg/display/gpu"

	"github.com/ygelfand/LANovo/internal/hardware/display"
)

var renderer = sharedgpu.New(display.Get())

func Get() *sharedgpu.Renderer { return renderer }
