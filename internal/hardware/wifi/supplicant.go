package wifi

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/layout"
)

// Android ships a supplicant but no way to use it without the framework, so init runs it against
// our own config and lanovod drives it over the control socket.
const (
	ConfigPath = layout.WifiConf
	SocketDir  = layout.WifiSockets
)

// Control is the supplicant's control interface.
type Control struct{ conn net.Conn }

// dials names each local socket apart from the last.
var dials atomic.Uint64

// dialsNoisy is how many control sockets a run opens before that is worth saying out loud.
const dialsNoisy = 1000

// Dial opens the control socket. wpa_supplicant replies to whatever address it was asked from, so
// the local end has to be one it can send back to.
//
// In the abstract namespace, so there is no file: one bound to a path has to be created, made
// writable by the supplicant's user, and unlinked afterwards, and anything that exits without
// unlinking leaves it behind for good. The supplicant answers an abstract address the same way,
// and it is what Android's own tools use.
func Dial() (*Control, error) {
	// A handful of these are opened over a run: one to listen on, one to ask with, a few while
	// waiting for the supplicant. Thousands means something is opening one per question.
	n := dials.Add(1)
	if n%dialsNoisy == 0 {
		slog.Warn("opening control sockets in bulk", "opened", n)
	}

	// Unique per connection: events are listened to on one while commands go over another, and
	// two binding the same name is one of them failing.
	local := fmt.Sprintf("@lanovod-%d-%d", os.Getpid(), n)

	conn, err := net.DialUnix("unixgram",
		&net.UnixAddr{Name: local, Net: "unixgram"},
		&net.UnixAddr{Name: filepath.Join(SocketDir, Interface), Net: "unixgram"})
	if err != nil {
		return nil, fmt.Errorf("wpa_supplicant control socket: %w", err)
	}
	return &Control{conn: conn}, nil
}

// Close releases the connection. Nothing to unlink: the local address is abstract and goes with
// the socket.
func (c *Control) Close() error { return c.conn.Close() }

// reply is how much of an answer is read.
//
// Big enough for a whole scan. This is a datagram socket, so a reply that does not fit is not
// continued in another read — the rest is dropped and nothing says so. SCAN_RESULTS in a block of
// flats runs well past the four kilobytes that is enough for every other command, and would come
// back looking like a shorter list rather than like an error.
const reply = 32 << 10

// Cmd sends one command and returns the reply.
func (c *Control) Cmd(cmd string) (string, error) {
	if err := c.conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return "", err
	}
	if _, err := c.conn.Write([]byte(cmd)); err != nil {
		return "", fmt.Errorf("%s: %w", cmd, err)
	}

	buf := make([]byte, reply)
	n, err := c.conn.Read(buf)
	if err != nil {
		return "", fmt.Errorf("%s: %w", cmd, err)
	}
	return strings.TrimSpace(string(buf[:n])), nil
}

// Status is what the supplicant reports, as key=value lines.
func (c *Control) Status() (map[string]string, error) {
	out, err := c.Cmd("STATUS")
	if err != nil {
		return nil, err
	}

	status := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			status[k] = v
		}
	}
	return status, nil
}

// Connect asks the supplicant to join a network it already has: one that has just started sits at
// DISCONNECTED until something asks.
//
// REASSOCIATE rather than RECONNECT: RECONNECT only acts on a supplicant that was told to
// DISCONNECT, and answers OK either way.
func Connect() error {
	c, err := Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	for _, cmd := range []string{"ENABLE_NETWORK all", "REASSOCIATE"} {
		out, err := c.Cmd(cmd)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(out, "OK") {
			return fmt.Errorf("wifi: %s said %q", cmd, out)
		}
	}
	return nil
}

// AwaitSupplicant opens init's supplicant, writing the configuration it needs if it is not there
// yet.
//
// Failing is how it waits: the radio is supervised, so a supplicant that is not up yet means a
// retry in a couple of seconds. A device with no configuration converges the same way — the
// supplicant fails for want of one, init starts it again, and by then this has written it.
func AwaitSupplicant() error {
	if c, err := Dial(); err == nil {
		c.Close()
		return nil
	}

	if err := os.MkdirAll(SocketDir, 0o770); err != nil {
		return fmt.Errorf("wifi: %w", err)
	}
	if _, err := os.Stat(ConfigPath); err != nil {
		const minimal = "ctrl_interface=" + SocketDir + "\nupdate_config=1\nbgscan=\"" + layout.WifiBgscan + "\"\n"
		if err := os.WriteFile(ConfigPath, []byte(minimal), 0o660); err != nil {
			return fmt.Errorf("wifi: writing %s: %w", ConfigPath, err)
		}
	}
	for path, mode := range map[string]os.FileMode{
		SocketDir:  0o770,
		ConfigPath: 0o660,
	} {
		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("wifi: %w", err)
		}
		if err := os.Chown(path, layout.WifiUser, layout.WifiUser); err != nil {
			return fmt.Errorf("wifi: %w", err)
		}
	}

	c, err := Dial()
	if err != nil {
		return fmt.Errorf("wifi: no supplicant on %s: %w", SocketDir, err)
	}
	c.Close()
	return nil
}
