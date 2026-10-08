package settings

import (
	sharedpreview "github.com/ygelfand/libcountertop/pkg/display/camerapreview"

	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

var cameraPreview = sharedpreview.New(
	sharedpreview.Dependencies{
		Shell:   shell.Get(),
		Camera:  livecam.Sessions(),
		Display: display.Get(),
	},
)
var camPage = cameraPreview.Page
var camWant = cameraPreview.Want
