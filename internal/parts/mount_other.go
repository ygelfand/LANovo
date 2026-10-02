//go:build !linux

package parts

import "errors"

func Writable() (restore func() error, err error) {
	return nil, errors.New("parts: only the device has a /system to remount")
}

func CopyLabel(from, to string) error { return nil }
