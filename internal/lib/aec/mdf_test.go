package aec

import (
	"math"
	"math/rand/v2"
	"strconv"
	"testing"
)

type processor interface {
	Process(mic, ref []int16) ([]int16, error)
	SetAdapting(on bool)
}

func runAny(t *testing.T, c processor, mic, ref []int16, holdOut int) (gotMic, gotOut []int16) {
	t.Helper()

	split := len(mic) - holdOut
	for i := 0; i+frame <= len(mic); i += frame {
		if i >= split {
			c.SetAdapting(false)
		}
		out, err := c.Process(mic[i:i+frame], ref[i:i+frame])
		if err != nil {
			t.Fatal(err)
		}
		if i >= split {
			gotOut = append(gotOut, out...)
			gotMic = append(gotMic, mic[i:i+frame]...)
		}
	}
	return gotMic, gotOut
}

func TestMDFCancelsAPureDelay(t *testing.T) {
	c, err := NewMDF(64, 1024, 16000)
	if err != nil {
		t.Fatal(err)
	}

	h := make([]float64, 60)
	h[57] = 0.5

	ref := noiseAt(48000, 8000, 1)
	gotMic, gotOut := runAny(t, c, echoed(ref, h), ref, 8000)
	// speexdsp's own mdf.c, built and run on this input, reaches 22.5 dB by the fourth second.
	if got := erle(gotMic, gotOut); got < 20 {
		t.Fatalf("erle %.1f dB on a pure delay, want at least 20", got)
	}
}

func TestMDFCancelsADispersiveTail(t *testing.T) {
	c, err := NewMDF(64, 1024, 16000)
	if err != nil {
		t.Fatal(err)
	}

	r := rand.New(rand.NewPCG(3, 9))
	h := make([]float64, 800)
	for k := 57; k < len(h); k++ {
		h[k] = (r.Float64()*2 - 1) * 0.4 * math.Exp(-float64(k-57)/200)
	}

	ref := noiseAt(160000, 8000, 2)
	gotMic, gotOut := runAny(t, c, echoed(ref, h), ref, 16000)
	if got := erle(gotMic, gotOut); got < 15 {
		t.Fatalf("erle %.1f dB on a dispersive tail, want at least 15", got)
	}
}

func BenchmarkMDFProcess(b *testing.B) {
	for _, taps := range []int{1024, 2048, 4096} {
		c, err := NewMDF(64, taps, 16000)
		if err != nil {
			b.Fatal(err)
		}

		mic, ref := noiseAt(frame, 6000, 1), noiseAt(frame, 8000, 2)

		b.Run(strconv.Itoa(taps), func(b *testing.B) {
			for range b.N {
				if _, err := c.Process(mic, ref); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/200000, "%core")
		})
	}
}

func TestMDFRejectsBadConfigAndLengths(t *testing.T) {
	if _, err := NewMDF(60, 1024, 16000); err == nil {
		t.Error("accepted a frame that is not a power of two")
	}
	if _, err := NewMDF(64, 64, 16000); err == nil {
		t.Error("accepted a tail of one frame")
	}
	c, err := NewMDF(64, 1024, 16000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Process(make([]int16, 100), make([]int16, 100)); err == nil {
		t.Error("accepted a length that is not a whole number of frames")
	}
	if _, err := c.Process(make([]int16, 128), make([]int16, 64)); err == nil {
		t.Error("accepted mismatched lengths")
	}
}
