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

// instance is the address a running lanovod holds for as long as it lives. Abstract, so there is
// no file to go stale: the kernel drops the name when the process does.
const instance = "@lanovod-instance"

// ExitTaken is what the process exits with when another lanovod already holds the device. Its own
// code, so init's logs separate this from having fallen over: one is a mistake, the other is a
// crash, and a service stuck in restarting looks the same either way.
const ExitTaken = 3

// Taken is another lanovod already running, and which one.
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

// Only takes the lock that says this is the only lanovod, and hands back what releases it.
//
// The framebuffer is not exclusive — two processes can open it and both draw, which shows up as a
// screen flicking between two things that are each individually correct. So is the sound card, the
// touchscreen and the supplicant socket. Nothing below notices; this is where it is caught.
func Only() (func(), error) {
	l, err := net.Listen("unix", instance)
	if err == nil {
		return func() { l.Close() }, nil
	}

	pid, name := holder()
	return nil, &Taken{Pid: pid, Name: name, Err: err}
}

// holder is the process holding the instance socket, or zero when it cannot be worked out.
//
// The abstract socket carries no pid, so this goes round: /proc/net/unix lists an inode against
// the name, and whichever process has that inode open is the one holding it. Worth the walk —
// "another lanovod is running" without saying which one has cost an hour twice.
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

// inodeOf is the inode /proc/net/unix lists against a socket name.
func inodeOf(name string) (string, bool) {
	f, err := os.Open("/proc/net/unix")
	if err != nil {
		return "", false
	}
	defer f.Close()

	return inodeIn(f, name)
}

// inodeIn reads the table apart from the file, so the parsing can be tested against a real one.
//
//	Num       RefCount Protocol Flags    Type St Inode Path
//	00000000: 00000002 00000000 00010000 0001 01 125021 @lanovod-instance
//
// An abstract name is printed with the leading NUL as @, which is how it is written here too. The
// match is exact: this device also has @lanovod and @lanovod-4943-3 open.
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

// holds reports whether a process has this socket open.
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

// comm is what a process calls itself, for saying which one it is.
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
