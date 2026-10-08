//go:build !linux

package wifi

import "errors"

func SetPowerSave(string, bool) error { return errors.New("wifi: only works on the device") }
