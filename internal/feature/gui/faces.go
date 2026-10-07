package gui

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/libcountertop/pkg/display/clockkind"
	"github.com/ygelfand/libcountertop/pkg/display/clockview"
)

type faceView = clockview.View

func clockView(kind config.Face) (faceView, bool) { return clockview.Of(clockkind.Kind(kind)) }

var lettering = clockview.Lettering
var fitted = clockview.Fitted
var imageSrc = clockview.ImageSource
