package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

// stat writes a /proc/stat with the given first line.
func stat(t *testing.T, line string) Reader {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "proc/stat"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	return Reader{Root: root}
}

// The fields are user, nice, system, idle, iowait, irq, softirq, steal. Idle and iowait are the
// two where the processor had nothing to run.
func TestCPUCountsIdleAndIowaitAsDoingNothing(t *testing.T) {
	// 10 + 0 + 20 busy, 60 + 10 idle: a hundred jiffies, thirty of them working.
	r := stat(t, "cpu  10 0 20 60 10 0 0 0\ncpu0 5 0 10 30 5 0 0 0\n")

	busy, total := r.CPU()
	if !busy.Known || !total.Known {
		t.Fatal("nothing was read")
	}
	if busy.Value != 30 {
		t.Errorf("busy = %v, want 30", busy.Value)
	}
	if total.Value != 100 {
		t.Errorf("total = %v, want 100", total.Value)
	}
}

// The first line is every core together; the per core lines after it are not what this reads.
func TestCPUReadsTheWholeMachineNotTheFirstCore(t *testing.T) {
	r := stat(t, "cpu  100 0 0 100 0 0 0 0\ncpu0 900 0 0 0 0 0 0 0\n")

	busy, total := r.CPU()
	if busy.Value != 100 || total.Value != 200 {
		t.Errorf(
			"read %v of %v, want 100 of 200 — it took a per core line",
			busy.Value,
			total.Value,
		)
	}
}

// Newer kernels add guest and guest_nice on the end. Summing whatever is there rather than the
// eight we know keeps that from quietly changing the denominator.
func TestCPUTakesHoweverManyFieldsThereAre(t *testing.T) {
	r := stat(t, "cpu  10 0 10 80 0 0 0 0 0 0\n")

	busy, total := r.CPU()
	if busy.Value != 20 || total.Value != 100 {
		t.Errorf("read %v of %v, want 20 of 100", busy.Value, total.Value)
	}
}

func TestCPUWithNothingToRead(t *testing.T) {
	for _, line := range []string{"", "cpu\n", "cpu 1 2 3\n", "cpu a b c d e\n", "intr 1 2 3\n"} {
		busy, total := stat(t, line).CPU()
		if busy.Known || total.Known {
			t.Errorf("%q read as %v of %v, want nothing", line, busy.Value, total.Value)
		}
	}
}

func TestCPUOnAMachineWithNoProcStat(t *testing.T) {
	busy, total := Reader{Root: t.TempDir()}.CPU()
	if busy.Known || total.Known {
		t.Error("a board with no /proc/stat reported a reading")
	}
}
