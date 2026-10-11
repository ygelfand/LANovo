package parts

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ygelfand/LANovo/internal/android/prop"
)

const blkroset = 0x125d

var byNameDirs = []string{"/dev/block/bootdevice/by-name", "/dev/block/platform/bootdevice/by-name"}

var writing sync.Mutex

func Writable() (restore func() error, err error) {
	writing.Lock()
	part, err := systemPartition()
	if err != nil {
		writing.Unlock()
		return nil, err
	}
	mount := systemMountpoint()
	if err := setReadOnly(part, false); err != nil {
		writing.Unlock()
		return nil, err
	}
	if err := syscall.Mount("", mount, "", syscall.MS_REMOUNT, ""); err != nil {
		_ = setReadOnly(part, true)
		writing.Unlock()
		return nil, fmt.Errorf("parts: remounting %s read-write: %w", mount, err)
	}
	return func() error {
		defer writing.Unlock()
		syscall.Sync()
		if err := syscall.Mount(
			"",
			mount,
			"",
			syscall.MS_REMOUNT|syscall.MS_RDONLY,
			"",
		); err != nil {
			return fmt.Errorf("parts: remounting %s read-only: %w", mount, err)
		}
		return setReadOnly(part, true)
	}, nil
}

// A system-as-root board carries the system image at /; a board with a standalone system partition mounts it at /system.
func systemMountpoint() string {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return "/"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[1] == "/system" {
			return "/system"
		}
	}
	return "/"
}

func systemPartition() (string, error) {
	slot, err := prop.Local.Getprop(prop.SlotSuffix)
	if err != nil {
		return "", err
	}
	for _, dir := range byNameDirs {
		part := dir + "/system" + slot
		if _, err := os.Stat(part); err == nil {
			return part, nil
		}
	}
	return "", fmt.Errorf("parts: no system%s partition link", slot)
}

func setReadOnly(part string, ro bool) error {
	f, err := os.OpenFile(part, os.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	v := int32(0)
	if ro {
		v = 1
	}
	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		blkroset,
		uintptr(unsafe.Pointer(&v)),
	); errno != 0 {
		return fmt.Errorf("parts: setting the read-only flag on %s: %w", part, errno)
	}
	return nil
}

func CopyLabel(from, to string) error {
	label := make([]byte, 256)
	n, err := syscall.Getxattr(from, "security.selinux", label)
	if err != nil {
		return err
	}
	return syscall.Setxattr(to, "security.selinux", label[:n], 0)
}
