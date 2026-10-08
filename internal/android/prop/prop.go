package prop

import (
	"fmt"
	"os/exec"
	"strings"
)

// PATH is not set up when init starts a service.
const (
	bin    = "/system/bin/setprop"
	getter = "/system/bin/getprop"
)

const (
	Model         = "ro.oem.product.model"
	BootCompleted = "sys.boot_completed"
	VerityMode    = "ro.boot.veritymode"
	SlotSuffix    = "ro.boot.slot_suffix"
)

func ServiceState(service string) string { return "init.svc." + service }

type Store interface {
	Getprop(name string) (string, error)
	Setprop(name, value string) error
}

var Local Store = local{}

type local struct{}

func (local) Getprop(name string) (string, error) {
	out, err := exec.Command(getter, name).Output()
	if err != nil {
		return "", fmt.Errorf("prop: %s %s: %w", getter, name, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ctl.stop marks the service stopped; init restarts a killed one.
func Stop(s Store, service string) error { return s.Setprop("ctl.stop", service) }

func Start(s Store, service string) error { return s.Setprop("ctl.start", service) }

func Restart(s Store, service string) error { return s.Setprop("ctl.restart", service) }

func Reboot(s Store) error { return s.Setprop("sys.powerctl", "reboot") }

func Running(s Store, service string) bool {
	v, err := s.Getprop(ServiceState(service))
	return err == nil && v == "running"
}

func Booted(s Store) bool {
	v, err := s.Getprop(BootCompleted)
	return err == nil && v == "1"
}
