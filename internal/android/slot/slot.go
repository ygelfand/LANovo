// Package slot tells the bootloader that the running boot worked.
//
// This device is A/B: two copies of the system, and a bootloader that boots one of them with a
// retry counter it decrements on every attempt. Something in userspace has to say the boot was good
// or the counter reaches zero, the bootloader marks the slot unbootable and starts the other one —
// which on this device is the stock slot, with no lanovod on it.
//
// On a stock Android that is update_engine's job. lanovod stops update_engine, so it is ours.
package slot

import (
	"fmt"
	"os"
	"syscall"
)

// bin is absolute because PATH is not set up when init starts a service.
const bin = "/system/bin/bootctl"

// MarkBooted tells the bootloader this slot is good, which resets its retry counter.
//
// Runs the device's own bootctl for the same reason prop runs setprop: what it does is write
// attribute bits into the GPT through the boot_control HAL, and a mistake there costs the partition
// table rather than a failed call.
func MarkBooted() error {
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("slot: %s: %w", bin, err)
	}

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
