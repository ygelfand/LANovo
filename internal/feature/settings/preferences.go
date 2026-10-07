package settings

import (
	"github.com/ygelfand/LANovo/internal/config"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
)

func preferences() sharedsettings.Preferences {
	return sharedsettings.Preferences{Screen: config.ScreenSection, Clock: config.ClockSection, Idle: config.IdleSection, Visual: config.VisualSection, Poster: config.PosterSection, Weather: config.WeatherSection, Wake: config.WakeSection, Call: config.CallSection}
}
