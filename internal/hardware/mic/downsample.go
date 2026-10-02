package mic

import "math"

// Voice is the rate speech recognition works at, and what the wake word models expect.
const Voice = 16000

// Decimation is how many captured frames make one at the voice rate: 48 kHz to 16 kHz.
const Decimation = Rate / Voice

const (
	taps   = 63
	cutoff = 6500.0
)

var kernel = design()

func design() [taps]float32 {
	var k [taps]float32
	var sum float64
	mid := float64(taps-1) / 2
	for i := range taps {
		x := float64(i) - mid
		s := 2 * cutoff / Rate
		if x != 0 {
			s = math.Sin(2*math.Pi*cutoff/Rate*x) / (math.Pi * x)
		}
		w := 0.42 - 0.5*math.Cos(2*math.Pi*float64(i)/(taps-1)) + 0.08*math.Cos(4*math.Pi*float64(i)/(taps-1))
		k[i] = float32(s * w)
		sum += s * w
	}
	for i := range k {
		k[i] = float32(float64(k[i]) / sum)
	}
	return k
}

type Downsampler struct {
	ring  [2 * taps]float32
	at    int
	phase int
}

func (d *Downsampler) Channel16k(samples []int16, ch int) []int16 {
	out := make([]int16, 0, len(samples)/(Channels*Decimation)+1)
	for i := ch; i < len(samples); i += Channels {
		out = d.push(out, float32(samples[i]))
	}
	return out
}

func (d *Downsampler) Of(mono []int16) []int16 {
	out := make([]int16, 0, len(mono)/Decimation+1)
	for _, v := range mono {
		out = d.push(out, float32(v))
	}
	return out
}

func (d *Downsampler) push(out []int16, m float32) []int16 {
	d.ring[d.at], d.ring[d.at+taps] = m, m
	d.at = (d.at + 1) % taps
	if d.phase++; d.phase < Decimation {
		return out
	}
	d.phase = 0
	window := d.ring[d.at : d.at+taps]
	var acc float32
	for k, v := range window {
		acc += kernel[k] * v
	}
	return append(out, int16(max(-32768, min(32767, acc))))
}
