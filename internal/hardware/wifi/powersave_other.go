//go:build !linux

package wifi

import "errors"

// Nothing off the device has a radio to set this on, and nl80211 is Linux only.
func SetPowerSave(string, bool) error { return errors.New("wifi: only works on the device") }
