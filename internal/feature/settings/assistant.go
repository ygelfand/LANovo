package settings

import (
	"github.com/ygelfand/libcountertop/pkg/assistant/turn"
	sharedsettings "github.com/ygelfand/libcountertop/pkg/display/settings"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/tones"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/wake"
)

func assistantPages() *sharedsettings.AssistantPages {
	return sharedsettings.NewAssistantPages(
		sharedsettings.AssistantDependencies{
			Slots:       wakeword.Slots,
			Preferences: preferences(),
			Models:      wake.Lib().Ours,
			Requested:   &wakeword.Requested,
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
