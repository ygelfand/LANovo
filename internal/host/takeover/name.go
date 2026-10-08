package takeover

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/layout"
)

var ErrNoName = errors.New("no device name")

func ReadName(d *adb.Device) (string, error) {
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

func WriteName(d *adb.Device, name string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	if _, err := d.Shell("mkdir -p " + layout.StateDir); err != nil {
		return err
	}
	return d.WriteFile(layout.NamePath, []byte(name+"\n"), 0o644)
}

func SuggestName(d *adb.Device) string {
	out, err := d.Shell("cat " + layout.MACPath)
	if err != nil {
		return layout.DefaultName
	}
	return layout.NameFromMAC(layout.MAC(out))
}

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
