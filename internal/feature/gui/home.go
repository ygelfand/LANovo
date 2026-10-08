package gui

import (
	_ "embed"

	gogui "github.com/go-gui-org/go-gui/gui"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/homeview"
	sharedhome "github.com/ygelfand/libcountertop/pkg/homeassistant/homecontrol"
	"github.com/ygelfand/libcountertop/pkg/say"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/settings"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

//go:embed esphome.svg
var esphomeLogo string

func homeViews() *sharedview.Renderer {
	return sharedview.New(
		sharedview.Dependencies{
			Engine:      homecontrol.Get(),
			Client:      homeassistant.Get(),
			Name:        func() string { return config.Get().Device.Name },
			Weather:     config.WeatherSection,
			WeatherPage: func() shell.View { return settings.WeatherPage() },
			Shell:       shell.Get(),
			Logo:        esphomeLogo,
			UI:          presentation,
		},
	)
}
func homeScreen(h *sharedhome.Home) *Screen {
	return &Screen{Title: say.T("home.title"), View: h, Build: homeBody}
}
func homeBody(w *gogui.Window) gogui.View { return homeViews().Body(w) }
func pickerScreen(p *sharedhome.Picker) *Screen {
	return &Screen{
		Title: p.Sel.Name(),
		View:  p,
		Build: func(*gogui.Window) gogui.View { return homeViews().Picker(p) },
	}
}
