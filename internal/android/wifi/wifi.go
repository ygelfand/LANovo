package wifi

import (
	"context"
	"fmt"
	"strings"

	"github.com/ygelfand/libcountertop/pkg/network/wpa"

	"github.com/ygelfand/LANovo/internal/host/device"
	"github.com/ygelfand/LANovo/internal/layout"
)

const (
	iface   = layout.WifiIface
	sockets = layout.WifiSockets
)

type (
	Security = wpa.Security
	Network  = wpa.Network
	State    = wpa.State
)

const (
	Open       = wpa.Open
	PSK        = wpa.PSK
	SAE        = wpa.SAE
	Enterprise = wpa.Enterprise
)

type shell struct{ d *device.Device }

func (s shell) Cmd(words ...string) (string, error) {
	cmd := fmt.Sprintf("wpa_cli -p %s -i %s %s", sockets, iface, strings.Join(words, " "))

	out, err := s.d.Shell(cmd)
	if err != nil {
		return out, err
	}
	if strings.Contains(out, "FAIL") {
		return out, fmt.Errorf("wifi: %s said %q", words[0], strings.TrimSpace(wpa.LastLine(out)))
	}
	return out, nil
}

func Scan(ctx context.Context, d *device.Device) ([]Network, error) {
	return wpa.Scanned(ctx, shell{d})
}

func Configured(d *device.Device) ([]string, error) { return wpa.Configured(shell{d}) }

func Remove(d *device.Device, ssid string) error { return wpa.Remove(shell{d}, ssid) }

func Join(d *device.Device, ssid, passphrase string) error {
	return wpa.Join(shell{d}, ssid, passphrase)
}

func Connect(d *device.Device) error { return wpa.Reconnect(shell{d}) }

func WPS(d *device.Device) error { return wpa.WPS(shell{d}) }

func Status(d *device.Device) (State, error) { return wpa.Status(shell{d}) }

func Wait(ctx context.Context, d *device.Device, ssid string) (State, error) {
	return wpa.Wait(ctx, shell{d}, ssid)
}
