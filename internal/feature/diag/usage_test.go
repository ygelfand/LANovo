package diag

import (
	"os"
	"path/filepath"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/hardware/metrics"
)

// at is a reader over a /proc/stat holding one moment's counters.
func at(t *testing.T, line string) metrics.Reader {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "proc/stat"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	return metrics.Reader{Root: root}
}

func watching() *Diag {
	return &Diag{usage: &esphome.Sensor{Base: esphome.Base{ObjectID: "cpu_usage"}}}
}

// The first sample has nothing to subtract, and reporting it anyway would publish the average
// since boot as if it were the last thirty seconds.
func TestTheFirstSampleReportsNothing(t *testing.T) {
	d := watching()
	d.busy(at(t, "cpu 100 0 100 800 0 0 0 0\n"))

	if got := d.usage.Get(); got != 0 {
		t.Errorf("the first sample published %v, want nothing", got)
	}
}

// What is wanted is the interval, not the whole time since boot. Between these two readings the
// board did 50 jiffies of work out of 100 elapsed.
func TestUsageIsTheDifferenceBetweenTwoReadings(t *testing.T) {
	d := watching()

	d.busy(at(t, "cpu 100 0 100 800 0 0 0 0\n")) // 200 busy of 1000
	d.busy(at(t, "cpu 125 0 125 850 0 0 0 0\n")) // 250 busy of 1100

	if got := d.usage.Get(); got != 50 {
		t.Errorf("usage = %v%%, want 50%%", got)
	}
}

// An idle interval reads as idle, which is the whole point: the load average on this board says
// three at the same moment.
func TestAnIdleIntervalReadsAsIdle(t *testing.T) {
	d := watching()

	d.busy(at(t, "cpu 100 0 100 800 0 0 0 0\n"))
	d.busy(at(t, "cpu 100 0 100 900 0 0 0 0\n"))

	if got := d.usage.Get(); got != 0 {
		t.Errorf("usage = %v%%, want 0%% on an interval where nothing ran", got)
	}
}

// Counters that went backwards mean the machine restarted under us, or a torn read. Neither is a
// measurement, and publishing a negative percentage is worse than publishing nothing.
func TestCountersGoingBackwardsPublishNothing(t *testing.T) {
	d := watching()

	d.busy(at(t, "cpu 500 0 500 5000 0 0 0 0\n"))
	d.busy(at(t, "cpu 10 0 10 100 0 0 0 0\n"))

	if got := d.usage.Get(); got != 0 {
		t.Errorf("a counter that went backwards published %v", got)
	}
}

// Two readings taken with nothing in between have no interval to describe.
func TestTwoIdenticalReadingsPublishNothing(t *testing.T) {
	d := watching()

	same := "cpu 100 0 100 800 0 0 0 0\n"
	d.busy(at(t, same))
	d.busy(at(t, same))

	if got := d.usage.Get(); got != 0 {
		t.Errorf("an interval of no time published %v", got)
	}
}

// A board with nothing to read leaves the sensor alone rather than reporting zero, which would
// read as an idle device instead of an unknown one.
func TestNoProcStatLeavesTheSensorAlone(t *testing.T) {
	d := watching()
	d.usage.Set(42)

	d.busy(metrics.Reader{Root: t.TempDir()})

	if got := d.usage.Get(); got != 42 {
		t.Errorf("usage = %v, want the last reading kept", got)
	}
}
