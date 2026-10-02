//go:build !linux

package dhcp

import (
	"context"
	"net"
)

// Nothing off the device holds an address worth defending, and packet sockets are Linux only.
func watching(context.Context, string, net.IP, func()) error { return nil }
