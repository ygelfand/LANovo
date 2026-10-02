//go:build !linux

package dhcp

import "net"

// probe says nothing is holding the address, which is the answer that takes it. Probing needs a
// packet socket, and off the device there is nothing to protect.
func probe(string, net.IP) (bool, error) { return false, nil }
