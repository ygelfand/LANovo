package mic

import (
	"encoding/binary"
	"testing"
)

func bytesOf(samples ...int16) []byte {
	b := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(s))
	}
	return b
}

func TestSamplesDecodesSignedLittleEndian(t *testing.T) {
	want := []int16{0, 1, -1, 32767, -32768, 1234}

	got := Samples(bytesOf(want...))
	if len(got) != len(want) {
		t.Fatalf("decoded %d samples, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sample %d = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestSamplesOfNothing(t *testing.T) {
	if got := Samples(nil); len(got) != 0 {
		t.Errorf("decoded %d samples from nothing", len(got))
	}
}

func TestChannel(t *testing.T) {
	frame := []int16{1, 100, 2, 200, 3, 300}

	left := Channel(frame, 0)
	right := Channel(frame, 1)

	if len(left) != 3 || left[0] != 1 || left[2] != 3 {
		t.Errorf("left = %v, want 1 2 3", left)
	}
	if len(right) != 3 || right[0] != 100 || right[2] != 300 {
		t.Errorf("right = %v, want 100 200 300", right)
	}
}

func TestChannelOutOfRange(t *testing.T) {
	frame := []int16{1, 2, 3, 4}

	if got := Channel(frame, -1); got != nil {
		t.Errorf("channel -1 gave %v", got)
	}
	if got := Channel(frame, Channels); got != nil {
		t.Errorf("channel %d gave %v", Channels, got)
	}
}

func TestPeak(t *testing.T) {
	tests := []struct {
		name    string
		samples []int16
		want    int
	}{
		{"silence", []int16{0, 0, 0, 0}, 0},
		{"a positive peak", []int16{10, 500, 20}, 500},
		{"a negative peak counts the same", []int16{10, -500, 20}, 500},
		{"full scale", []int16{0, 32767}, 32767},
		{"nothing at all", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Peak(tt.samples); got != tt.want {
				t.Errorf("Peak(%v) = %d, want %d", tt.samples, got, tt.want)
			}
		})
	}
}

func TestIdentical(t *testing.T) {
	tests := []struct {
		name    string
		samples []int16
		want    bool
	}{
		{"one mic copied", []int16{5, 5, 9, 9, -3, -3}, true},
		{"two mics", []int16{5, 6, 9, 2, -3, -3}, false},
		{"silence from both is identical", []int16{0, 0, 0, 0}, true},
		{"one differing pair is enough", []int16{5, 5, 9, 9, 1, 2}, false},
		{"nothing to compare", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Identical(tt.samples); got != tt.want {
				t.Errorf("Identical(%v) = %v, want %v", tt.samples, got, tt.want)
			}
		})
	}
}

func TestGainIsClamped(t *testing.T) {
	m := Get()

	for _, tt := range []struct {
		in, want int
	}{
		{-50, MinGain},
		{0, MinGain},
		{84, 84},
		{124, MaxGain},
		{9999, MaxGain},
	} {
		if err := m.SetGain(tt.in); err != nil {
			t.Fatalf("SetGain(%d): %v", tt.in, err)
		}
		if got := m.Gain(); got != tt.want {
			t.Errorf("SetGain(%d) left the gain at %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestRouteSetsTheTertiaryChannels(t *testing.T) {
	var found bool
	for _, s := range qualcommRoute {
		if s.name == "TERT_MI2S_TX Channels" {
			found = true
			if s.choice != "Two" {
				t.Errorf("TERT_MI2S_TX Channels is set to %q, want Two", s.choice)
			}
		}
	}
	if !found {
		t.Error("the route does not set TERT_MI2S_TX Channels, so capture will be mono")
	}
}

func TestRouteSetsBothDecimators(t *testing.T) {
	want := map[string]string{"DEC1 MUX": "DMIC1", "DEC2 MUX": "DMIC2"}

	for _, s := range qualcommRoute {
		if choice, ok := want[s.name]; ok {
			if s.choice != choice {
				t.Errorf("%s is set to %q, want %q", s.name, s.choice, choice)
			}
			delete(want, s.name)
		}
	}
	for name := range want {
		t.Errorf("the route does not set %s", name)
	}
}

func TestA32BitCaptureKeepsTheTopSixteenBits(t *testing.T) {
	b := []byte{0x00, 0x9c, 0xfc, 0xff, 0x00, 0x31, 0x01, 0x00}
	got := samplesOf(b, 32)
	if len(got) != 2 || got[0] != -4 || got[1] != 1 {
		t.Errorf("samplesOf = %v, want [-4 1]", got)
	}
}
