package takeover

import (
	"fmt"
	"strings"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/host/device"
)

// Qualcomm's partition links, then MediaTek's.
var byNameDirs = []string{"/dev/block/bootdevice/by-name", "/dev/block/platform/bootdevice/by-name"}

func ByName(d *device.Device) (string, error) {
	for _, dir := range byNameDirs {
		if _, err := d.Shell("ls -d " + dir); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no partition links in %s", strings.Join(byNameDirs, " or "))
}

// remount,rw succeeds and does nothing while the block device is BLKROSET; sysfs ro is 0444.
func Writable(d *device.Device) (restore func() error, err error) {
	slot, err := d.Getprop(prop.SlotSuffix)
	if err != nil {
		return nil, err
	}
	dir, err := ByName(d)
	if err != nil {
		return nil, err
	}
	part := dir + "/system" + slot

	if _, err := d.Shell("blockdev --setrw " + part); err != nil {
		return nil, fmt.Errorf("clearing the read-only flag on %s: %w", part, err)
	}
	if _, err := d.Shell("mount -o remount,rw /"); err != nil {
		return nil, fmt.Errorf("remounting / read-write: %w", err)
	}
	if rw, err := isWritable(d); err != nil || !rw {
		return nil, fmt.Errorf("/ did not become writable")
	}

	return func() error {
		// The remount can report busy while writes are outstanding.
		_, _ = d.Shell("sync")
		if _, err := d.Shell("mount -o remount,ro /"); err != nil {
			return fmt.Errorf("remounting / read-only: %w", err)
		}
		_, err := d.Shell("blockdev --setro " + part)
		return err
	}, nil
}

func isWritable(d *device.Device) (bool, error) {
	out, err := d.Shell("grep -E ' / ' /proc/mounts")
	if err != nil {
		return false, err
	}
	fields := strings.Fields(out)
	if len(fields) < 4 {
		return false, fmt.Errorf("unreadable mount line: %q", out)
	}
	return strings.HasPrefix(fields[3], "rw"), nil
}
