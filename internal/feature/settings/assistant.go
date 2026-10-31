package settings

import (
	"github.com/ygelfand/libcountertop/pkg/assistant/turn"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"
	sharedha "github.com/ygelfand/libcountertop/pkg/homeassistant"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/detect"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/tones"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/lib/wake"
)

func assistantPages() *sharedsettings.AssistantPages {
	return sharedsettings.NewAssistantPages(
		sharedsettings.AssistantDependencies{
			Slots:       wakeword.Slots,
			Pryon:       detect.Get().Pryon(),
			PryonStop:   detect.Get().Pryon().Stop(),
			Gemini:      detect.Get().Gemini(),
			Preferences: preferences(),
			Models:      wake.Lib().Ours,
			Phrases:     wake.Lib().Phrases,
			Chooser: sharedha.WakeWords{
				Controller: homeassistant.Get(),
				MAC:        func() string { return wifi.Get().MAC() },
			},
			Pipelines: sharedha.Pipelines{
				Controller: homeassistant.Get(),
				MAC:        func() string { return wifi.Get().MAC() },
			},
			Requested: &wakeword.Requested,
			Words: turn.Words{
				Settings: config.WakeSection,
				Speaker:  speaker.Sound(),
				Sounds:   tones.Get(),
			},
			Look:  lookPages(),
			Shell: shell.Get(),
		},
	)
}
