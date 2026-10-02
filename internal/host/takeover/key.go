package takeover

import (
	"errors"
	"fmt"
	"strings"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/host/device"
	"github.com/ygelfand/LANovo/internal/layout"
)

// ErrNoKey means there is no key on the device, which is what unprovisioned looks like: it comes
// up on the reserved zero key and Home Assistant sets one when it adds the device.
var ErrNoKey = errors.New("the device has no encryption key")

// Key is the encryption key Home Assistant pairs with, read off the device.
//
// One look, not a wait. Nothing on the device mints a key any more — it is written by Home
// Assistant setting one, or by RotateKey — so a key that is absent now will still be absent in a
// minute.
func Key(d *device.Device) (string, error) {
	b, err := d.ReadFile(layout.KeyPath)
	if err != nil {
		return "", ErrNoKey
	}
	return parseKey(b)
}

// parseKey is what counts as a key, separated from the device so it can be tested against the
// shapes a file can actually be in.
//
// Checked rather than passed through. A truncated or half-written key printed as if it were the
// real one costs more than printing nothing: it gets copied into Home Assistant, refused, and
// blamed on everything except the thing that printed it.
func parseKey(b []byte) (string, error) {
	key := strings.TrimSpace(string(b))
	if key == "" {
		return "", ErrNoKey
	}

	if _, err := esphome.ParsePSK(key); err != nil {
		return "", fmt.Errorf("the key at %s is not one: %w", layout.KeyPath, err)
	}
	return key, nil
}

// RotateKey replaces the key with a new one and restarts the service to take it up.
//
// Deliberately not part of an install. Home Assistant holds the old key, and a device whose key
// changes under it stops connecting until somebody puts the new one in — which is a thing to do on
// purpose, not a thing to have happen while installing an update.
func RotateKey(d *device.Device) (string, error) {
	k, err := esphome.GeneratePSK()
	if err != nil {
		return "", fmt.Errorf("generating a key: %w", err)
	}

	// 0600 and root's, the same as lanovod writes it: /data is the device's own storage and the
	// key is the whole of the device's security.
	if err := d.WriteFile(layout.KeyPath, []byte(k.String()+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("writing %s: %w", layout.KeyPath, err)
	}

	if _, err := d.Shell("setprop ctl.restart " + layout.Service); err != nil {
		return "", err
	}
	return k.String(), nil
}
