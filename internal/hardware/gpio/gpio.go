package gpio

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var Base = "/sys/class/gpio"

const (
	In  = "in"
	Out = "out"
)

type Pin struct{ N int }

func (p Pin) dir() string             { return filepath.Join(Base, fmt.Sprintf("gpio%d", p.N)) }
func (p Pin) path(attr string) string { return filepath.Join(p.dir(), attr) }

func (p Pin) Exported() bool {
	_, err := os.Stat(p.dir())
	return err == nil
}

func (p Pin) Export() error {
	if p.Exported() {
		return nil
	}
	if err := write(filepath.Join(Base, "export"), strconv.Itoa(p.N)); err != nil {
		return fmt.Errorf("gpio: exporting %d: %w", p.N, err)
	}
	return nil
}

func (p Pin) Unexport() error {
	if !p.Exported() {
		return nil
	}
	return write(filepath.Join(Base, "unexport"), strconv.Itoa(p.N))
}

func (p Pin) SetDirection(d string) error {
	if err := write(p.path("direction"), d); err != nil {
		return fmt.Errorf("gpio: %d direction %s: %w", p.N, d, err)
	}
	return nil
}

func (p Pin) Direction() (string, error) {
	v, err := read(p.path("direction"))
	if err != nil {
		return "", fmt.Errorf("gpio: %d direction: %w", p.N, err)
	}
	return v, nil
}

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

func (p Pin) Get() (bool, error) {
	v, err := read(p.path("value"))
	if err != nil {
		return false, fmt.Errorf("gpio: %d: %w", p.N, err)
	}
	return v == "1", nil
}

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
