package visuals

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/visualview"
)

type View = sharedview.View

func (v *Visuals) View() *View {
	return sharedview.New(v, sharedview.Options{
		Open: gpu.Open, Orientation: func() int { return int(display.Get().Orientation()) },
		Visible: shell.Get().Visible, MaxFPS: func() int { return config.Get().Visual.MaxFPS },
	})
}
