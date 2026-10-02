package control

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/hardware/mic"
)

func TestTheEchoCaptureKeepsItsChannelsInOrder(t *testing.T) {
	const period = 320
	var frames []mic.EchoFrame
	for p := range 20 {
		f := mic.EchoFrame{Cancelling: p%2 == 0, Aligned: true, Offset: 7}
		ch := make([][]int16, echoChannels)
		for c := range ch {
			ch[c] = make([]int16, period)
			for i := range ch[c] {
				ch[c][i] = int16(1000*(c+1) + p)
			}
		}
		f.Mic = [2][]int16{ch[0], ch[1]}
		f.Reference = ch[2]
		f.Out = [2][]int16{ch[3], ch[4]}
		frames = append(frames, f)
	}

	path := filepath.Join(t.TempDir(), "echo.wav")
	say, err := echoReport(frames, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"aligned true offset 7", "cancelling 0.20s", "samples 6400"} {
		if !strings.Contains(say, want) {
			t.Errorf("report lacks %q: %s", want, say)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := 44 + 20*period*echoChannels*2; len(data) != want {
		t.Fatalf("wav is %d bytes, want %d", len(data), want)
	}
	for c := range echoChannels {
		got := int16(binary.LittleEndian.Uint16(data[44+c*2:]))
		if want := int16(1000 * (c + 1)); got != want {
			t.Errorf("channel %d starts at %d, want %d", c+1, got, want)
		}
	}
}
