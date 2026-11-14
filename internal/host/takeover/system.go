package takeover

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ygelfand/libcountertop/pkg/host/adb"

	"github.com/ygelfand/LANovo/internal/android/prop"
)

// Qualcomm's partition links, then MediaTek's.
var byNameDirs = []string{"/dev/block/bootdevice/by-name", "/dev/block/platform/bootdevice/by-name"}

func ByName(d *adb.Device) (string, error) {
	for _, dir := range byNameDirs {
		if _, err := d.Shell("ls -d " + dir); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no partition links in %s", strings.Join(byNameDirs, " or "))
}

// A system-as-root board carries the system image at /; a board with a standalone system partition mounts it at /system.
func systemMount(d *adb.Device) (string, error) {
	out, err := d.Shell("cat /proc/mounts")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[1] == "/system" {
			return "/system", nil
		}
	}
	return "/", nil
}

// remount,rw succeeds and does nothing while the block device is BLKROSET; sysfs ro is 0444.
func Writable(d *adb.Device, retryBusy bool) (restore func() error, err error) {
	slot, err := d.Getprop(prop.SlotSuffix)
	if err != nil {
		return nil, err
	}
	dir, err := ByName(d)
	if err != nil {
		return nil, err
	}
	part := dir + "/system" + slot

	mount, err := systemMount(d)
	if err != nil {
		return nil, err
	}

	if _, err := d.Shell("blockdev --setrw " + part); err != nil {
		return nil, fmt.Errorf("clearing the read-only flag on %s: %w", part, err)
	}
	if _, err := d.Shell("mount -o remount,rw " + mount); err != nil {
		return nil, fmt.Errorf("remounting %s read-write: %w", mount, err)
	}
	if rw, err := isWritable(d, mount); err != nil || !rw {
		return nil, fmt.Errorf("%s did not become writable", mount)
	}

	return func() error {
		return restoreReadOnly(d, mount, part, retryBusy, time.Sleep)
	}, nil
}

var errRemountBusy = errors.New("filesystem busy after sync")

type shellRunner interface {
	Shell(string) (string, error)
}

func restoreReadOnly(d shellRunner, mount, part string, retryBusy bool, pause func(time.Duration)) error {
	if _, err := d.Shell("sync"); err != nil && retryBusy {
		return fmt.Errorf("sync before remount: %w", err)
	}
	for attempt := 1; ; attempt++ {
		if _, err := d.Shell("mount -o remount,ro " + mount); err != nil {
			mountErr := fmt.Errorf("remounting %s read-only: %w", mount, err)
			if !retryBusy || !strings.Contains(strings.ToLower(err.Error()), "device or resource busy") {
				return mountErr
			}
			if attempt == 3 {
				return errors.Join(errRemountBusy, mountErr)
			}
			pause(time.Second)
			continue
		}
		if retryBusy {
			if err := requireReadOnly(d, mount); err != nil {
				return err
			}
		}
		_, err := d.Shell("blockdev --setro " + part)
		return err
	}
}

// CheckSystemReadOnly confirms the active system mount after an install reboot.
func CheckSystemReadOnly(d *adb.Device) error {
	mount, err := systemMount(d)
	if err != nil {
		return err
	}
	return requireReadOnly(d, mount)
}

func requireReadOnly(d shellRunner, mount string) error {
	options, err := mountOptions(d, mount)
	if err != nil {
		return err
	}
	if !slices.Contains(options, "ro") {
		return fmt.Errorf("%s is still mounted read-write", mount)
	}
	return nil
}

func isWritable(d *adb.Device, mount string) (bool, error) {
	out, err := d.Shell("grep -E ' " + mount + " ' /proc/mounts")
	if err != nil {
		return false, err
	}
	fields := strings.Fields(out)
	if len(fields) < 4 {
		return false, fmt.Errorf("unreadable mount line: %q", out)
	}
	return strings.HasPrefix(fields[3], "rw"), nil
}

func mountOptions(d shellRunner, mount string) ([]string, error) {
	out, err := d.Shell("cat /proc/mounts")
	if err != nil {
		return nil, err
	}
	for line := range strings.Lines(out) {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[1] == mount {
			return strings.Split(fields[3], ","), nil
		}
	}
	return nil, fmt.Errorf("mount %s missing from /proc/mounts", mount)
}
