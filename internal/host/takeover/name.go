package takeover

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ygelfand/LANovo/internal/host/device"
	"github.com/ygelfand/LANovo/internal/layout"
)

// ErrNoName means neither the device nor the caller supplied a name.
var ErrNoName = errors.New("no device name")

// ReadName is the device's configured name, empty when it has none.
func ReadName(d *device.Device) (string, error) {
	have, err := d.Exists(layout.NamePath)
	if err != nil || !have {
		return "", err
	}

	b, err := d.ReadFile(layout.NamePath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// WriteName records what Home Assistant calls the device.
func WriteName(d *device.Device, name string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	if _, err := d.Shell("mkdir -p " + layout.StateDir); err != nil {
		return err
	}
	return d.WriteFile(layout.NamePath, []byte(name+"\n"), 0o644)
}

// SuggestName derives a default from the device's address, unique per device.
func SuggestName(d *device.Device) string {
	out, err := d.Shell("cat " + layout.MACPath)
	if err != nil {
		return layout.DefaultName
	}
	return layout.NameFromMAC(layout.MAC(out))
}

// ValidName checks the display name can produce a usable node name. The name is stored as typed
// and shown in Home Assistant; the node name it slugifies to becomes the mDNS hostname and the
// entity id prefix.
func ValidName(name string) error {
	slug := layout.Slug(name)
	if slug == "" {
		return fmt.Errorf("name %q has no letters or digits to build a hostname from", name)
	}
	if len(slug) > layout.MaxNodeName {
		return fmt.Errorf("name %q becomes %q, %d characters, and the limit is %d",
			name, slug, len(slug), layout.MaxNodeName)
	}
	return nil
}
