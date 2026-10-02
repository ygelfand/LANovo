package aec

import "errors"

const (
	full    = 32768
	erleTau = 8000
)

var ErrLength = errors.New("aec: mic and ref must be the same length")
