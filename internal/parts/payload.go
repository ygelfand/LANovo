//go:build payload

package parts

import _ "embed"

//go:embed payload/lanovo-surface
var surface []byte

//go:embed payload/liblanovo-camshim.so
var camshim []byte

//go:embed payload/lanovo-camera
var camera []byte

//go:embed payload/lanovo-voice
var voice []byte

//go:embed payload/lanovo-airplay
var airplay []byte
