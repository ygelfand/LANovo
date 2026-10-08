package settings

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/camera/live"
	sharedpreview "github.com/ygelfand/libcountertop/pkg/display/camerapreview"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/rtspd"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

var cameraPages = sync.OnceValue(func() *sharedsettings.CameraPages[live.Knobs] {
	cam := livecam.Get()
	preview := sharedpreview.New(sharedpreview.Dependencies{
		Shell:   shell.Get(),
		Camera:  cam.Sessions(),
		Display: display.Get(),
	})
	return sharedsettings.NewCameraPages(sharedsettings.CameraOptions[live.Knobs]{
		Table:     cam.Table,
		Read:      cam.Saved,
		Save:      cam.Set,
		Reset:     cam.Reset,
		Stream:    func() bool { return config.Get().RTSP.Enabled },
		SetStream: rtspd.Get().SetEnabled,
		Push:      shell.Get().Push,
		Redraw:    shell.Get().Redraw,
		Preview:   preview.Want,
		Wrap:      preview.Page,
	})
})

func cameraPage() *sharedshell.Page { return cameraPages().Page() }
