package ble

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
)

// Where the kernel keeps the radio switches. The Bluetooth one here is bt_power, a driver whose
// only job is the chip's enable line: there is no Bluetooth stack behind it, so unblocking it
// powers the part and nothing else happens.
const rfkillDir = "/sys/class/rfkill"

// settle is how long the chip is given after being let out of reset. It comes up on its own clock
// and will not answer before then, and a version request into a chip that is still starting reads
// as a chip that is not there.
const settle = 100 * time.Millisecond

// Power finds the Bluetooth kill switch and takes it off, so the chip is running when the UART is
// opened. Reports whether it had to change anything.
//
// Idempotent, because the way this gets used is running the bring-up again after it went wrong,
// and power-cycling the chip halfway through that is its own confusion.
func Power(on bool) (changed bool, err error) {
	if board.Current().SoC == board.MediaTek {
		return false, nil
	}
	at, err := killSwitch()
	if err != nil {
		return false, err
	}

	// Soft is the one userspace owns. Hard is a physical switch, and a device that has one wired
	// says so here rather than by staying silent once the rest is right.
	if hard, err := reads(filepath.Join(at, "hard")); err == nil && hard == "1" {
		return false, fmt.Errorf("ble: the radio is blocked by something physical")
	}

	was, err := reads(filepath.Join(at, "soft"))
	if err != nil {
		return false, err
	}

	want := "1"
	if on {
		want = "0"
	}
	if was == want {
		return false, nil
	}

	if err := os.WriteFile(filepath.Join(at, "soft"), []byte(want), 0o644); err != nil {
		return false, fmt.Errorf("ble: %s the radio: %w", turning(on), err)
	}
	if on {
		time.Sleep(settle)
	}
	return true, nil
}

func turning(on bool) string {
	if on {
		return "unblocking"
	}
	return "blocking"
}

// killSwitch is the rfkill directory for Bluetooth. By type rather than by number, because the
// numbering follows the order the drivers registered and the wifi one is next to it.
func killSwitch() (string, error) {
	entries, err := os.ReadDir(rfkillDir)
	if err != nil {
		return "", fmt.Errorf("ble: no rfkill: %w", err)
	}

	for _, e := range entries {
		at := filepath.Join(rfkillDir, e.Name())
		if kind, err := reads(filepath.Join(at, "type")); err == nil && kind == "bluetooth" {
			return at, nil
		}
	}
	return "", fmt.Errorf("ble: nothing in %s is a bluetooth radio", rfkillDir)
}

func reads(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
