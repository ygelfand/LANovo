//go:build unix

package prop

import (
	"fmt"
	"syscall"
)

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
