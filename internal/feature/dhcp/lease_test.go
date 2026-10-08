package dhcp

import (
	"path/filepath"
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

	if got := hostname(); got != fallback {
		t.Errorf("hostname = %q, want %q", got, fallback)
	}
}

func TestHostnameWithANameThatSlugsToNothing(t *testing.T) {
	config.Started(config.Device{Name: "!!!"})

	if got := hostname(); got != fallback {
		t.Errorf("hostname = %q, want %q", got, fallback)
	}
}

func TestTheAddressIsKeptInTheConfig(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	if err := (saved{}).SetAddress("10.0.0.7"); err != nil {
		t.Fatalf("saving: %v", err)
	}
	if got := config.Get().Network.Address; got != "10.0.0.7" {
		t.Errorf("the config says %q, want 10.0.0.7", got)
	}
	if got := (saved{}).Address(); got != "10.0.0.7" {
		t.Errorf("read back %q", got)
	}
}
