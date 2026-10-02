//go:build !linux

package i2c

import "errors"

func Open(int) (Bus, error) { return nil, errors.New("i2c: only works on the device") }

func OpenPath(string) (Bus, error) { return nil, errors.New("i2c: only works on the device") }
