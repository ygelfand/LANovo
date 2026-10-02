//go:build !linux

package dhcp

import "errors"

// Nothing off the device has an interface to put a lease on, and rtnetlink is Linux only.
func apply(string, *Lease) error { return errors.New("dhcp: only works on the device") }

func release(string) {}
