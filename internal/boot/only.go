package boot

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

// The kernel drops an abstract socket name when its process exits.
const instance = "@lanovod-instance"

const ExitTaken = 3

type Taken struct {
	Pid  int
	Name string
	Err  error
}

func (t *Taken) Error() string {
	if t.Pid == 0 {
		return fmt.Sprintf("another lanovod holds %s, and nothing in /proc owns up to it: %v",
			instance, t.Err)
	}
	return fmt.Sprintf("another lanovod holds %s: pid %d, %s", instance, t.Pid, t.Name)
}

func (t *Taken) Unwrap() error { return t.Err }

// The framebuffer, sound card, touchscreen and supplicant socket are not exclusive.
func Only() (func(), error) {
	l, err := net.Listen("unix", instance)
	if err == nil {
		return func() { _ = l.Close() }, nil
	}

	pid, name := holder()
	return nil, &Taken{Pid: pid, Name: name, Err: err}
}

// /proc/net/unix lists an abstract socket's inode, not its pid.
func holder() (pid int, name string) {
	inode, ok := inodeOf(instance)
	if !ok {
		return 0, ""
	}
	socket := "socket:[" + inode + "]"

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, ""
	}

	mine := os.Getpid()
	for _, e := range entries {
		at, err := strconv.Atoi(e.Name())
		if err != nil || at == mine {
			continue
		}
		if holds(at, socket) {
			return at, comm(at)
		}
	}
	return 0, ""
}

func inodeOf(name string) (string, bool) {
	f, err := os.Open("/proc/net/unix")
	if err != nil {
		return "", false
	}
	defer f.Close()

	return inodeIn(f, name)
}

// /proc/net/unix prints an abstract name's leading NUL as @.
func inodeIn(r io.Reader, name string) (string, bool) {
	const (
		inodeAt = 6
		pathAt  = 7
	)

	in := bufio.NewScanner(r)
	for in.Scan() {
		fields := strings.Fields(in.Text())
		if len(fields) <= pathAt || fields[pathAt] != name {
			continue
		}
		if _, err := strconv.Atoi(fields[inodeAt]); err != nil {
			continue
		}
		return fields[inodeAt], true
	}
	return "", false
}

func holds(pid int, socket string) bool {
	dir := "/proc/" + strconv.Itoa(pid) + "/fd"

	fds, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, fd := range fds {
		if at, err := os.Readlink(dir + "/" + fd.Name()); err == nil && at == socket {
			return true
		}
	}
	return false
}

func comm(pid int) string {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return "unknown"
	}
	if name := strings.TrimSpace(string(b)); name != "" {
		return name
	}
	return "unknown"
}
