package sendspin

import (
	"path/filepath"
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
	esphome "github.com/ygelfand/go-esphome-device"
)

// The same property every other setting is held to: the setter reaches the file and the entity
// both, and a command from Home Assistant reaches the file.
//
// This one is why the Features page exists. The switch was a Home Assistant entity and nothing
// else, so a device whose Home Assistant is down could not be set up from its own screen.
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

// Off by default. It opens a port and advertises itself, which is not something a device should
// start doing because nobody said otherwise.
func TestItIsOffOnAFreshDevice(t *testing.T) {
	if config.Defaults().Sendspin.Enabled {
		t.Error("a device nobody has set up is listening for a server")
	}
}
