package slot

import (
	"fmt"
	"os"
	"syscall"
)

// PATH is not set up when init starts a service.
const bin = "/system/bin/bootctl"

// bootctl writes GPT attribute bits through the boot_control HAL.
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
