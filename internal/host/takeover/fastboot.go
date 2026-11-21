package takeover

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ygelfand/libcountertop/pkg/host/fastboot"
)

const unlockGuide = "https://xdaforums.com/t/lenovo-smart-clock-bootloader-avb-unlock-firmware-region-changer-kernel-source.4130295/"

func FastbootChecks(fb *fastboot.Device) ([]Check, error) {
	unlocked, err := fb.Getvar("unlocked")
	if err != nil {
		return nil, err
	}
	secure, err := fb.Getvar("secure")
	if err != nil {
		return nil, err
	}
	return []Check{
		{
			Name:  "unlocked",
			Got:   unlocked,
			Want:  "yes",
			OK:    unlocked == "yes",
			Fatal: true,
			Fix:   "unlock the bootloader first — " + unlockGuide,
		},
		{
			Name:  "secure",
			Got:   secure,
			Want:  "no",
			OK:    secure == "no",
			Fatal: true,
			Fix:   "turn secure boot off — " + unlockGuide,
		},
	}, nil
}

func FlashRecovery(ctx context.Context, fb *fastboot.Device, image []byte) (string, error) {
	slot, err := fb.Getvar("current-slot")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(slot) == "" {
		return "", fmt.Errorf("the bootloader names no current slot")
	}
	slot = normalizeSlot(slot)

	tmp, err := os.CreateTemp("", "lanovo-boot-*.img")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(image); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	part := "boot" + slot
	if err := fb.Flash(part, tmp.Name()); err != nil {
		return "", fmt.Errorf("flashing %s: %w", part, err)
	}
	if err := fb.OEM("reboot-recovery"); err != nil {
		return "", fmt.Errorf("rebooting to recovery: %w", err)
	}
	return part, nil
}

func normalizeSlot(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "_") {
		return s
	}
	return "_" + s
}
