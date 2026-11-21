package slot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"syscall"

	"github.com/ygelfand/LANovo/internal/android/prop"
)

// PATH is not set up when init starts a service.
const bin = "/system/bin/bootctl"

func MarkBooted() error {
	suffix, err := prop.Local.Getprop(prop.SlotSuffix)
	if err == nil && suffix == "" {
		return nil
	}
	if _, serr := os.Stat(bin); serr == nil {
		return markViaBootctl()
	}
	if err != nil {
		return fmt.Errorf("slot: %w", err)
	}
	return markViaMisc(suffix)
}

// bootctl writes GPT attribute bits through the boot_control HAL.
func markViaBootctl() error {
	pid, err := syscall.ForkExec(
		bin,
		[]string{"bootctl", "mark-boot-successful"},
		&syscall.ProcAttr{
			Files: []uintptr{0, 1, 2},
		},
	)
	if err != nil {
		return fmt.Errorf("slot: %s: %w", bin, err)
	}

	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
		return err
	}
	if status.ExitStatus() != 0 {
		return fmt.Errorf("slot: bootctl mark-boot-successful: exit %d", status.ExitStatus())
	}
	return nil
}

// MediaTek keeps A/B slot metadata in misc at 0x800; the boot_control HAL is present but bootctl is not.
const miscCtrlOffset = 0x800

var miscByName = []string{
	"/dev/block/platform/bootdevice/by-name/misc",
	"/dev/block/bootdevice/by-name/misc",
}

var miscMagic = []byte{0x00, 0x41, 0x42, 0x30}

func markViaMisc(suffix string) error {
	var idx int
	switch suffix {
	case "_a":
		idx = 0
	case "_b":
		idx = 1
	default:
		return fmt.Errorf("slot: unexpected slot suffix %q", suffix)
	}

	path, err := miscPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("slot: %s: %w", path, err)
	}
	defer f.Close()

	var ctrl [32]byte
	if _, err := f.ReadAt(ctrl[:], miscCtrlOffset); err != nil {
		return fmt.Errorf("slot: reading %s: %w", path, err)
	}
	if !bytes.Equal(ctrl[0:4], miscMagic) || ctrl[4] > 1 {
		return fmt.Errorf("slot: %s holds no known A/B control", path)
	}
	if binary.BigEndian.Uint32(ctrl[28:32]) != crc32.ChecksumIEEE(ctrl[0:28]) {
		return fmt.Errorf("slot: the A/B control in %s fails its checksum", path)
	}

	slot := 8 + idx*4
	priority, tries, successful := ctrl[slot], ctrl[slot+1], ctrl[slot+2]
	if priority == 0 || (successful == 0 && tries == 0) {
		return fmt.Errorf("slot: %q is not bootable", suffix)
	}
	if tries == 0 && successful == 1 {
		return nil
	}
	ctrl[slot+1] = 0
	ctrl[slot+2] = 1
	binary.BigEndian.PutUint32(ctrl[28:32], crc32.ChecksumIEEE(ctrl[0:28]))

	if _, err := f.WriteAt(ctrl[:], miscCtrlOffset); err != nil {
		return fmt.Errorf("slot: writing %s: %w", path, err)
	}
	return f.Sync()
}

func miscPath() (string, error) {
	for _, p := range miscByName {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("slot: no misc partition link in %v", miscByName)
}
