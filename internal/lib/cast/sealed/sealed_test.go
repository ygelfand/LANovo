package sealed

import (
	"bytes"
	"testing"

	"github.com/ygelfand/LANovo/internal/lib/surface"
)

func TestTheWidevinePSSHIsChosenOverOthers(t *testing.T) {
	box := func(system [16]byte) []byte {
		b := make([]byte, 32)
		copy(b[4:], "pssh")
		copy(b[12:], system[:])
		return b
	}
	playready := [16]byte{0x9a, 0x04, 0xf0, 0x79}
	wv := box(surface.Widevine)
	if got := widevine([][]byte{box(playready), wv}); !bytes.Equal(got, wv) {
		t.Errorf("chose %x", got)
	}
	if got := widevine([][]byte{box(playready)}); got != nil {
		t.Errorf("chose %x with no Widevine box", got)
	}
}
