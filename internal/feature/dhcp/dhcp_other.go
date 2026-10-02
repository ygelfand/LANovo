//go:build !linux

package dhcp

import (
	"context"
	"errors"
)

// Nothing off the device has an interface to take a lease on, and DHCP needs AF_PACKET, which is
// Linux only.
func dhcpRequest(context.Context, string) (*Lease, error) {
	return nil, errors.New("dhcp: only works on the device")
}
