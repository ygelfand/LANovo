// Package gpio drives the lines through sysfs.
//
// Lenovo's app held these and is gone, so nothing exports them any more and lanovod does it
// itself. The numbering is the kernel's: the SoC's lines start at 0 and the PMIC's at 1016.
package gpio

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Base is where the kernel puts them. A variable so a test can point it at a directory.
var Base = "/sys/class/gpio"

// The directions a line can be in.
const (
	In  = "in"
	Out = "out"
)

// Pin is one line.
type Pin struct{ N int }

func (p Pin) dir() string             { return filepath.Join(Base, fmt.Sprintf("gpio%d", p.N)) }
func (p Pin) path(attr string) string { return filepath.Join(p.dir(), attr) }

// Exported reports whether the kernel has already given us the line.
func (p Pin) Exported() bool {
	_, err := os.Stat(p.dir())
	return err == nil
}

// Export asks for the line. Exporting one that is already exported is not an error: something
// else may have had it first, and that is the normal case after a restart.
func (p Pin) Export() error {
	if p.Exported() {
		return nil
	}
	if err := write(filepath.Join(Base, "export"), strconv.Itoa(p.N)); err != nil {
		return fmt.Errorf("gpio: exporting %d: %w", p.N, err)
	}
	return nil
}

// Unexport gives it back.
func (p Pin) Unexport() error {
	if !p.Exported() {
		return nil
	}
	return write(filepath.Join(Base, "unexport"), strconv.Itoa(p.N))
}

// SetDirection points the line in or out.
func (p Pin) SetDirection(d string) error {
	if err := write(p.path("direction"), d); err != nil {
		return fmt.Errorf("gpio: %d direction %s: %w", p.N, d, err)
	}
	return nil
}

// Direction is which way it is pointing.
func (p Pin) Direction() (string, error) {
	v, err := read(p.path("direction"))
	if err != nil {
		return "", fmt.Errorf("gpio: %d direction: %w", p.N, err)
	}
	return v, nil
}

// Set drives an output.
func (p Pin) Set(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	if err := write(p.path("value"), v); err != nil {
		return fmt.Errorf("gpio: %d = %s: %w", p.N, v, err)
	}
	return nil
}

// Get reads the line.
func (p Pin) Get() (bool, error) {
	v, err := read(p.path("value"))
	if err != nil {
		return false, fmt.Errorf("gpio: %d: %w", p.N, err)
	}
	return v == "1", nil
}

// Output exports a line and points it out, which is every use of one that drives something.
func Output(n int) (Pin, error) {
	p := Pin{N: n}
	if err := p.Export(); err != nil {
		return p, err
	}
	return p, p.SetDirection(Out)
}

func read(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func write(path, v string) error { return os.WriteFile(path, []byte(v), 0o644) }
