package settings

import (
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/lib/surface"
	"github.com/ygelfand/LANovo/internal/ui"
	sharedpreview "github.com/ygelfand/libcountertop/pkg/display/camerapreview"
)

var cameraPreview = sharedpreview.New(sharedpreview.Options{
	Shell:  shell.Get(),
	Helper: func() *surface.Client { return display.Get().Helper() },
	Select: func() (int, int, int) {
		sizes := livecam.Sizes()
		if len(sizes) == 0 {
			return -1, 0, 0
		}
		at := len(sizes) - 1
		return at, sizes[at].Width, sizes[at].Height
	},
	Join: livecam.Join, Leave: livecam.Leave,
	Orientation: func() int { return int(display.Get().Orientation()) },
	Place: func(box ui.Rect, w, h int) (float32, float32, surface.Matrix) {
		fw, fh := display.Get().Native()
		x, y, m, _ := display.Get().Orientation().Place(fw, fh, display.Rect{X: box.X, Y: box.Y, W: box.W, H: box.H}, w, h)
		return x, y, m
	},
})
var camPage = cameraPreview.Page
var camWant = cameraPreview.Want
