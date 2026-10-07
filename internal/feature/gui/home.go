package gui

import (
	_ "embed"

	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/homecontrol"
	"github.com/ygelfand/LANovo/internal/feature/settings"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/homeview"
	"github.com/ygelfand/libcountertop/pkg/say"
)

//go:embed esphome.svg
var esphomeLogo string

func homeViews() *sharedview.Renderer {
	return sharedview.New(
		sharedview.Options{
			Engine:      homecontrol.Engine(),
			Client:      homeassistant.Get(),
			Name:        func() string { return config.Get().Device.Name },
			Weather:     func() config.Weather { return config.Get().Weather },
			WeatherPage: func() shell.View { return settings.WeatherPage() },
			Shell:       shell.Get(),
			Logo:        esphomeLogo,
			Row:         rowView,
			Input:       textField,
			Toolkit:     toolkit,
		},
	)
}
func homeScreen(h *homecontrol.Home) *Screen {
	return &Screen{Title: say.T("home.title"), View: h, Build: homeBody}
}
func homeBody(w *gogui.Window) gogui.View { return homeViews().Body(w) }
func pickerScreen(p *homecontrol.Picker) *Screen {
	return &Screen{
		Title: p.Sel.Name(),
		View:  p,
		Build: func(*gogui.Window) gogui.View { return homeViews().Picker(p) },
	}
}
func pickerBody(p *homecontrol.Picker) gogui.View { return homeViews().Picker(p) }

var column = sharedview.Column
var separator = sharedview.Separator
