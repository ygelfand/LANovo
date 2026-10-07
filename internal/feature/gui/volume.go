package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
)

func volumeScreen(v shell.View) *Screen { return &Screen{View: v, Build: volumeCard} }
func volumeCard(w *gogui.Window) gogui.View {
	vol := volume.Get()
	stream, open := vol.Picked()
	var choices []widgets.VolumeChoice
	for _, s := range config.Streams() {
		choices = append(
			choices,
			widgets.VolumeChoice{Key: string(s), Label: s.Label(), Level: vol.Level(s)},
		)
	}
	return toolkit.VolumeCard(w, widgets.VolumeOptions{
		Current: widgets.VolumeChoice{
			Key:   string(stream),
			Label: stream.Label(),
			Level: vol.Level(stream),
		},
		Choices: choices,
		Open:    open,
		Left:    config.Get().Screen.Volume == config.EdgeLeft,
		Set:     func(v int) { vol.Set(stream, v) },
		Pick:    func(s string) { vol.Pick(config.Stream(s)) },
		Linger:  vol.Linger,
		Expand:  vol.Expand,
		Dismiss: vol.Dismiss,
		Grip:    grip,
	})
}
