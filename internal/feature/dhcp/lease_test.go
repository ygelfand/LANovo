package dhcp

import (
	"github.com/ygelfand/LANovo/internal/config"
	"testing"
)

// The name someone gave the device is what it asks the DHCP server to call it, slugged the way
// the ESPHome node name is, so a lookup by either finds the same machine.
func TestHostnameIsTheDeviceName(t *testing.T) {
	config.Started(config.Device{Name: "Kitchen Display"})

	if got := hostname(); got != "kitchen-display" {
		t.Errorf("hostname = %q, want the device name slugged", got)
	}
}

// A device nobody has named still has to ask for something, and the product is a better answer
// than this board's kernel hostname, which is localhost.
func TestHostnameFallsBackToTheProduct(t *testing.T) {
	config.Started(config.Device{})

	if got := hostname(); got != Fallback {
		t.Errorf("hostname = %q, want %q", got, Fallback)
	}
}

// A name of nothing but punctuation slugs to nothing, which is not a hostname to ask for.
func TestHostnameWithANameThatSlugsToNothing(t *testing.T) {
	config.Started(config.Device{Name: "!!!"})

	if got := hostname(); got != Fallback {
		t.Errorf("hostname = %q, want %q", got, Fallback)
	}
}
