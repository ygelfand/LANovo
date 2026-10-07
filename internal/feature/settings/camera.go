package settings

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/rtspd"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/setting"
	"github.com/ygelfand/LANovo/internal/ui/widget"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
)

var cameraOnce sync.Once
var cameraSettings *sharedsettings.CameraPages[livecam.Knobs]

func cameraPages() *sharedsettings.CameraPages[livecam.Knobs] {
	cameraOnce.Do(func() {
		cameraSettings = sharedsettings.NewCameraPages(
			sharedsettings.CameraOptions[livecam.Knobs]{
				Table:     livecam.Table,
				Read:      livecam.Saved,
				Save:      livecam.Set,
				Reset:     livecam.Reset,
				Stream:    func() bool { return config.Get().RTSP.Enabled },
				SetStream: rtspd.Get().SetEnabled,
				Push:      shell.Get().Push,
				Redraw:    shell.Get().Redraw,
				Preview:   camWant,
				Wrap:      camPage,
			},
		)
	})
	return cameraSettings
}
func cameraPage() *shell.Page                 { return cameraPages().Page() }
func sectionPage(g setting.Group) *shell.Page { return cameraPages().Section(g) }
func choicePage(s livecam.Knob) *shell.Page   { return cameraPages().Choice(s) }
func cameraRow(s livecam.Knob, k *livecam.Knobs) (widget.Row, func(int)) {
	return cameraPages().Row(s, k)
}
func shown() []setting.Group { return cameraPages().Shown() }

var dimmed = sharedsettings.Dimmed
