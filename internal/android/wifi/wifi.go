// Package wifi configures the device's wifi through wpa_cli, which ships on the device.
//
// The protocol is internal/lib/wpa; this is the adb transport for it. The supplicant owns the
// configuration and writes it itself, so nothing here composes a wpa_supplicant.conf. Android's
// framework reads its networks back from the supplicant at startup, so one added this way also
// shows up in Settings.
package wifi

import (
	"context"
	"fmt"
	"strings"

	"github.com/ygelfand/LANovo/internal/host/device"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/wpa"
)

const (
	iface   = layout.WifiIface
	sockets = layout.WifiSockets
)

// Security, Network and State are the protocol's, re-exported so callers of this package do not
// have to know that.
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

// shell is wpa_cli over adb.
type shell struct{ d *device.Device }

// Cmd runs one wpa_cli command on the device.
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

// Scan lists what the device can see, strongest first, one entry per name.
func Scan(ctx context.Context, d *device.Device) ([]Network, error) {
	return wpa.Scanned(ctx, shell{d})
}

// Configured is the names the supplicant already has.
func Configured(d *device.Device) ([]string, error) { return wpa.Configured(shell{d}) }

// Remove forgets every network with this name.
func Remove(d *device.Device, ssid string) error { return wpa.Remove(shell{d}, ssid) }

// Join adds a network and selects it. An empty passphrase means an open network.
func Join(d *device.Device, ssid, passphrase string) error {
	return wpa.Join(shell{d}, ssid, passphrase)
}

// Connect asks the supplicant to join a network it already has.
func Connect(d *device.Device) error { return wpa.Reconnect(shell{d}) }

// WPS joins by push button, for a router that has one.
func WPS(d *device.Device) error { return wpa.WPS(shell{d}) }

// Status reads the current connection.
func Status(d *device.Device) (State, error) { return wpa.Status(shell{d}) }

// Wait blocks until the device has joined this network. An empty ssid waits for any network.
func Wait(ctx context.Context, d *device.Device, ssid string) (State, error) {
	return wpa.Wait(ctx, shell{d}, ssid)
}
