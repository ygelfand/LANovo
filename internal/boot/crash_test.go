package boot

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
)

func TestTheLastCrashIsKeptAndTheNextStartsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crash")
	report := "fatal error: concurrent map read and map write\n\ngoroutine 1 [running]:\n"
	if err := os.WriteFile(path, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}

	keepCrashes(path)
	t.Cleanup(func() { _ = debug.SetCrashOutput(nil, debug.CrashOptions{}) })

	if b, err := os.ReadFile(path + ".last"); err != nil || string(b) != report {
		t.Errorf("the last report reads %q, %v", b, err)
	}
	if b, err := os.ReadFile(path); err != nil || len(b) != 0 {
		t.Errorf("the new report holds %q, %v", b, err)
	}
}

func TestAnEmptyReportLeavesTheLastOneAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crash")
	if err := os.WriteFile(path+".last", []byte("panic: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	keepCrashes(path)
	t.Cleanup(func() { _ = debug.SetCrashOutput(nil, debug.CrashOptions{}) })

	if b, _ := os.ReadFile(path + ".last"); string(b) != "panic: old\n" {
		t.Errorf("a clean run replaced the last report with %q", b)
	}
}

func TestTheFatalLineIsFound(t *testing.T) {
	for report, want := range map[string]string{
		"fatal error: all goroutines are asleep\n\ngoroutine 1:\n":     "fatal error: all goroutines are asleep",
		"\npanic: runtime error: index out of range\n\ngoroutine 7:\n": "panic: runtime error: index out of range",
		"runtime: out of memory\n":                                     "runtime: out of memory",
	} {
		if got := fatalLine([]byte(report)); got != want {
			t.Errorf("%q gives %q, want %q", report, got, want)
		}
	}
}
