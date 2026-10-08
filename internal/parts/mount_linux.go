package parts

import (
	"fmt"
	"os"
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
	if err := setReadOnly(part, false); err != nil {
		writing.Unlock()
		return nil, err
	}
	if err := syscall.Mount("", "/", "", syscall.MS_REMOUNT, ""); err != nil {
		_ = setReadOnly(part, true)
		writing.Unlock()
		return nil, fmt.Errorf("parts: remounting / read-write: %w", err)
	}
	return func() error {
		defer writing.Unlock()
		syscall.Sync()
		if err := syscall.Mount("", "/", "", syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("parts: remounting / read-only: %w", err)
		}
		return setReadOnly(part, true)
	}, nil
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
