package buttons

import (
	"os"
	"path/filepath"
	"testing"
)

// value writes what sysfs would hold for a line.
func value(t *testing.T, v string) *os.File {
	t.Helper()

	path := filepath.Join(t.TempDir(), "value")
	if err := os.WriteFile(path, []byte(v), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// The camera shutter is the one that reads the other way round: 1 means open, and open is not
// engaged. Getting this backwards reports a covered camera as watching.
func TestPolarity(t *testing.T) {
	tests := []struct {
		name string
		line line
		held string
		want bool
	}{
		{"a button pressed", line{VolumeUp, 85, false}, "1\n", true},
		{"a button released", line{VolumeUp, 85, false}, "0\n", false},
		{"the mic slider over", line{MicMute, 86, false}, "1\n", true},
		{"the mic slider back", line{MicMute, 86, false}, "0\n", false},
		{"the shutter closed reads 0", line{CameraCover, 87, true}, "0\n", true},
		{"the shutter open reads 1", line{CameraCover, 87, true}, "1\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.line.read(value(t, tt.held)); got != tt.want {
				t.Errorf("read = %v, want %v", got, tt.want)
			}
		})
	}
}

// sysfs values carry a newline, and some kernels pad them.
func TestReadIgnoresWhitespace(t *testing.T) {
	l := line{VolumeUp, 85, false}

	for _, held := range []string{"1", "1\n", "1\r\n", " 1 \n"} {
		if !l.read(value(t, held)) {
			t.Errorf("a line holding %q read as released", held)
		}
	}
}

// read seeks to the start each time, because the same file is read again on every edge and a
// file left at EOF would report nothing forever.
func TestReadTwiceGivesTheSameAnswer(t *testing.T) {
	l := line{VolumeUp, 85, false}
	f := value(t, "1\n")

	if !l.read(f) {
		t.Fatal("the first read said released")
	}
	if !l.read(f) {
		t.Error("the second read of the same file said released")
	}
}

// Every control the device has is named and mapped to a line, and no two share a GPIO.
func TestLines(t *testing.T) {
	if len(lines()) != 4 {
		t.Fatalf("%d lines, want the four controls", len(lines()))
	}

	seen := map[int]Button{}
	for _, l := range lines() {
		if l.button == "" {
			t.Errorf("gpio %d has no name", l.gpio)
		}
		if was, ok := seen[l.gpio]; ok {
			t.Errorf("gpio %d is both %s and %s", l.gpio, was, l.button)
		}
		seen[l.gpio] = l.button
	}
}

// The volume down key is on the PMIC rather than the SoC, which is why its number is nothing like
// the others.
func TestVolumeDownIsOnThePMIC(t *testing.T) {
	for _, l := range lines() {
		if l.button == VolumeDown && l.gpio != 1019 {
			t.Errorf("volume down is gpio %d, want 1019 on the PMIC", l.gpio)
		}
	}
}
