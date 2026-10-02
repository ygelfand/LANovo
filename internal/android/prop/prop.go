// Package prop sets Android system properties.
//
// The property service's socket protocol is private to init and libcutils, so this runs the
// device's own setprop. It happens a handful of times at start-up and once for a reboot, which is
// not worth reimplementing a private protocol for.
package prop

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// Absolute because PATH is not set up when init starts a service.
const (
	bin    = "/system/bin/setprop"
	getter = "/system/bin/getprop"
)

// The properties read by name.
const (
	Model         = "ro.oem.product.model"
	BootCompleted = "sys.boot_completed"
	VerityMode    = "ro.boot.veritymode"
	SlotSuffix    = "ro.boot.slot_suffix"
)

// ServiceState is init's view of a service: running, stopped, restarting.
func ServiceState(service string) string { return "init.svc." + service }

// Store is somewhere properties live: this device, or one at the other end of adb.
type Store interface {
	Getprop(name string) (string, error)
	Setprop(name, value string) error
}

// Local is this device's own.
var Local Store = local{}

type local struct{}

// Setprop writes a property.
func (local) Setprop(name, value string) error {
	pid, err := syscall.ForkExec(bin, []string{"setprop", name, value}, &syscall.ProcAttr{
		Files: []uintptr{0, 1, 2},
	})
	if err != nil {
		return fmt.Errorf("prop: %s %s %s: %w", bin, name, value, err)
	}

	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
		return err
	}
	if status.ExitStatus() != 0 {
		return fmt.Errorf("prop: setprop %s %s: exit %d", name, value, status.ExitStatus())
	}
	return nil
}

// Getprop reads a property. An unset one is empty, which is what getprop prints for it.
func (local) Getprop(name string) (string, error) {
	out, err := exec.Command(getter, name).Output()
	if err != nil {
		return "", fmt.Errorf("prop: %s %s: %w", getter, name, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Stop asks init to stop a service. Killing the pid would just make init restart it; ctl.stop
// marks it stopped and init leaves it alone.
func Stop(s Store, service string) error { return s.Setprop("ctl.stop", service) }

// Start asks init to start a service, which also clears a stop it was marked with.
func Start(s Store, service string) error { return s.Setprop("ctl.start", service) }

// Restart asks init to stop a service and start it again. One step, rather than a stop that has
// to be undone by whoever is left running.
func Restart(s Store, service string) error { return s.Setprop("ctl.restart", service) }

// Reboot brings the whole device back through init, which unwinds the services it started rather
// than dropping the device where it stands.
func Reboot(s Store) error { return s.Setprop("sys.powerctl", "reboot") }

// Running is whether init has the service up.
func Running(s Store, service string) bool {
	v, err := s.Getprop(ServiceState(service))
	return err == nil && v == "running"
}

// Booted is whether Android has finished coming up.
func Booted(s Store) bool {
	v, err := s.Getprop(BootCompleted)
	return err == nil && v == "1"
}
