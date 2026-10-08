package sendspin

import (
	"path/filepath"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/config"
)

func TestEnablingTellsBothSides(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	p := Get()

	for _, want := range []bool{true, false, true} {
		p.SetEnabled(want)

		if got := config.Get().Sendspin.Enabled; got != want {
			t.Errorf("the file says %v, want %v", got, want)
		}
		if got := p.Entities()[0].(*esphome.Switch).Get(); got != want {
			t.Errorf("the entity says %v, want %v", got, want)
		}
	}
}

func TestTheCommandFromHomeAssistantIsSaved(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	p := Get()

	p.Entities()[0].(*esphome.Switch).OnCommand(true)
	if !config.Get().Sendspin.Enabled {
		t.Error("turning it on from Home Assistant did not save")
	}

	p.Entities()[0].(*esphome.Switch).OnCommand(false)
	if config.Get().Sendspin.Enabled {
		t.Error("turning it off from Home Assistant did not save")
	}
}

func TestItIsOffOnAFreshDevice(t *testing.T) {
	if config.Defaults().Sendspin.Enabled {
		t.Error("a device nobody has set up is listening for a server")
	}
}
