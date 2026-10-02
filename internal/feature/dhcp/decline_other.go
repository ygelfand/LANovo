//go:build !linux

package dhcp

import (
	"errors"
	"net"
)

// sendDecline needs a packet socket. Off the device there is no lease to decline.
func sendDecline(string, net.IP, net.IP, string) error {
	return errors.New("dhcp: declining only works on the device")
}
