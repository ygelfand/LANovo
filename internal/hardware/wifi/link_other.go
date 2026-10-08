//go:build !linux

package wifi

import "errors"

func linkUp(string) error { return errors.New("wifi: only works on the device") }
