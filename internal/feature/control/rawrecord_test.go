package control

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/hardware/mic"
)

func TestTheRawReportReadsEachMicAndWhetherTheyAgree(t *testing.T) {
	frames := mic.Rate / 10
	in := make([]int16, frames*mic.Channels)
	for i := range frames {
		v := int16(3277 * math.Sin(2*math.Pi*1000*float64(i)/mic.Rate))
		in[i*2], in[i*2+1] = v, -v/10
	}
	path := filepath.Join(t.TempDir(), "raw.wav")
	say, err := rawReport(in, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"mic1 peakdbfs -20.0", "mic2 peakdbfs -40.0", "correlation -1.00"} {
		if !strings.Contains(say, want) {
			t.Errorf("report %q is missing %q", say, want)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := int(b[22]) | int(b[23])<<8; got != mic.Channels {
		t.Errorf("the wav says %d channels", got)
	}
	if len(b) != 44+len(in)*2 {
		t.Errorf("the wav is %d bytes, want %d", len(b), 44+len(in)*2)
	}
}
