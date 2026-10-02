//go:build !linux

package dhcp

import (
	"errors"
	"net"
)

// Nothing off the device has an interface to announce on, and packet sockets are Linux only.
func announce(string, net.IP) error { return errors.New("dhcp: only works on the device") }
