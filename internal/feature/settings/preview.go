package settings

import (
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	sharedpreview "github.com/ygelfand/libcountertop/pkg/display/camerapreview"
)

var cameraPreview = sharedpreview.New(sharedpreview.Dependencies{Shell: shell.Get(), Camera: livecam.Sessions(), Display: display.Get()})
var camPage = cameraPreview.Page
var camWant = cameraPreview.Want
