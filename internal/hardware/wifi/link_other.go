//go:build !linux

package wifi

import "errors"

// Nothing off the device has an interface to bring up, and rtnetlink is Linux only.
func linkUp(string) error { return errors.New("wifi: only works on the device") }
