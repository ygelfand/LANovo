package dhcp

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/config"
)

func TestHostnameIsTheDeviceName(t *testing.T) {
	config.Started(config.Device{Name: "Kitchen Display"})

	if got := hostname(); got != "kitchen-display" {
		t.Errorf("hostname = %q, want the device name slugged", got)
	}
}

func TestHostnameFallsBackToTheProduct(t *testing.T) {
	config.Started(config.Device{})

	if got := hostname(); got != Fallback {
		t.Errorf("hostname = %q, want %q", got, Fallback)
	}
}

func TestHostnameWithANameThatSlugsToNothing(t *testing.T) {
	config.Started(config.Device{Name: "!!!"})

	if got := hostname(); got != Fallback {
		t.Errorf("hostname = %q, want %q", got, Fallback)
	}
}
