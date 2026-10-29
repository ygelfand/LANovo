package commands

import (
	"sync"

	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/voice"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/discovery"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/parts"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(41))
}

type Commands struct{ *voice.Listener }

func (c *Commands) Restore(config.Config) { c.Apply() }

func (c *Commands) Close() error {
	c.Stop()
	return nil
}

var get = sync.OnceValue(func() *Commands {
	c := &Commands{voice.NewListener(voice.Options{
		Helper:   layout.SpeechHelper,
		Binary:   parts.Speech(),
		ModelDir: layout.ModelDir,
		Source:   mic.Get(),
		Paused:   func() bool { return privacy.Get().MicMuted() || call.Get().PausesWake() },
		Settings: config.VoiceSection,
		Home:     homeassistant.Get(),
		Peers:    discovery.Get().PeerNames,
		Retired: []string{
			layout.StateDir + "/bin/lanovo-voice",
			layout.ModelDir + "/whistle-*.cact",
		},
	})}
	return c
})

func Get() *Commands { return get() }
