package gpu

import (
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/lib/surface"
	"github.com/ygelfand/LANovo/internal/ui"
	sharedgpu "github.com/ygelfand/libcountertop/pkg/display/gpu"
)

type Layer = sharedgpu.Layer

var ErrNoHelper = sharedgpu.ErrNoHelper
var renderer = sharedgpu.New(sharedgpu.Options{
	Helper: func() *surface.Client { return display.Get().Helper() },
	Place: func(at ui.Rect, w, h int) (float32, float32, surface.Matrix) {
		d := display.Get()
		fw, fh := d.Native()
		x, y, m, _ := d.Orientation().
			Place(fw, fh, display.Rect{X: at.X, Y: at.Y, W: at.W, H: at.H}, w, h)
		return x, y, m
	},
})
var Open = renderer.Open
var OpenAt = renderer.OpenAt
var OpenOffscreen = renderer.OpenOffscreen
