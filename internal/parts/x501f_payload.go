//go:build payload && x501fpayload

package parts

import _ "embed"

//go:embed payload/x501f-hal-tone
var x501fAudio []byte

//go:embed payload/x501f-audio-policy
var x501fAudioPolicy []byte
