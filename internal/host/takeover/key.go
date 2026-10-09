package takeover

import (
	"errors"
	"fmt"
	"strings"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/layout"
)

var ErrNoKey = errors.New("the device has no encryption key")

func Key(d *adb.Device) (string, error) {
	have, err := d.Exists(layout.KeyPath)
	if err != nil {
		return "", err
	}
	if !have {
		return "", ErrNoKey
	}
	b, err := d.ReadFile(layout.KeyPath)
	if err != nil {
		return "", ErrNoKey
	}
	return parseKey(b)
}

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

func RotateKey(d *adb.Device) (string, error) {
	k, err := esphome.GeneratePSK()
	if err != nil {
		return "", fmt.Errorf("generating a key: %w", err)
	}

	if err := d.WriteFile(layout.KeyPath, []byte(k.String()+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("writing %s: %w", layout.KeyPath, err)
	}

	if _, err := d.Shell("setprop ctl.restart " + layout.Service); err != nil {
		return "", err
	}
	return k.String(), nil
}
