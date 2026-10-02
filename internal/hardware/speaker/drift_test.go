package speaker

import (
	"math"
	"testing"
	"time"
)

func TestDriftHoldsStillWhileSettling(t *testing.T) {
	var d Drift
	at := time.Unix(0, 0)
	for i := range 40 {
		if ppm := d.Observe(1000+i*100, at.Add(time.Duration(i)*100*time.Millisecond)); ppm != 0 {
			t.Fatalf("%v in: skew %v before settling", time.Duration(i)*100*time.Millisecond, ppm)
		}
	}
}

func TestDriftLeansAgainstAQueueThatGrowsOrShrinks(t *testing.T) {
	for _, c := range []struct {
		name string
		by   int
		sign float64
	}{{"growing", 4800, 1}, {"shrinking", -4800, -1}} {
		var d Drift
		at := time.Unix(0, 0)
		for i := 0; i <= 60; i++ {
			d.Observe(9600, at.Add(time.Duration(i)*100*time.Millisecond))
		}
		var ppm float64
		for i := 61; i <= 400; i++ {
			ppm = d.Observe(9600+c.by, at.Add(time.Duration(i)*100*time.Millisecond))
		}
		if ppm*c.sign <= 0 {
			t.Errorf("%s queue: skew %v, want sign %v", c.name, ppm, c.sign)
		}
		if math.Abs(ppm) > driftMost {
			t.Errorf("%s queue: skew %v past the cap", c.name, ppm)
		}
	}
}

func TestDriftIsCapped(t *testing.T) {
	var d Drift
	at := time.Unix(0, 0)
	for i := 0; i <= 60; i++ {
		d.Observe(0, at.Add(time.Duration(i)*100*time.Millisecond))
	}
	var ppm float64
	for i := 61; i <= 2000; i++ {
		ppm = d.Observe(Rate*60, at.Add(time.Duration(i)*100*time.Millisecond))
	}
	if ppm != driftMost {
		t.Errorf("a minute of excess gave %v, want the cap %v", ppm, driftMost)
	}
}

func TestSkewMovesTheRatioByWhatItSays(t *testing.T) {
	for _, from := range []int{44100, 48000} {
		in := make([]int16, from*10*2)
		count := func(ppm float64) int {
			r := NewSkewable(from, 48000, 2)
			r.Skew(ppm)
			n := 0
			for at := 0; at < len(in); at += 1024 {
				n += len(r.Run(in[at:min(at+1024, len(in))])) / 2
			}
			return n
		}

		plain := count(0)
		for _, ppm := range []float64{500, -500, 37} {
			got := float64(count(ppm)-plain) / float64(plain) * 1e6
			if math.Abs(got+ppm) > 20 {
				t.Errorf("from %d: skew %v moved the output by %.0f ppm, want %v", from, ppm, got, -ppm)
			}
		}
	}
}

func TestASkewableSameRateStreamPassesATone(t *testing.T) {
	const n = 48000
	in := make([]int16, n*2)
	for i := range n {
		v := int16(12000 * math.Sin(2*math.Pi*1000*float64(i)/48000))
		in[2*i], in[2*i+1] = v, v
	}
	r := NewSkewable(48000, 48000, 2)
	var out []int16
	for at := 0; at < len(in); at += 960 {
		out = append(out, r.Run(in[at:min(at+960, len(in))])...)
	}
	if d := len(out)/2 - n; d < -1 || d > 1 {
		t.Fatalf("%d frames out of %d in", len(out)/2, n)
	}
	var sum, ref float64
	for i := 4800; i < n-100; i++ {
		sum += float64(out[2*i]) * float64(out[2*i])
		ref += float64(in[2*i]) * float64(in[2*i])
	}
	if ratio := math.Sqrt(sum / ref); ratio < 0.95 || ratio > 1.05 {
		t.Errorf("a 1 kHz tone came through at %.3f of its level", ratio)
	}
}
