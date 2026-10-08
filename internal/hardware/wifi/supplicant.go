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

const (
	ConfigPath = layout.WifiConf
	SocketDir  = layout.WifiSockets
)

type Control struct{ conn net.Conn }

var dials atomic.Uint64

const dialsNoisy = 1000

// wpa_supplicant replies to whatever address it was asked from.
func Dial() (*Control, error) {
	n := dials.Add(1)
	if n%dialsNoisy == 0 {
		slog.Warn("opening control sockets in bulk", "opened", n)
	}

	local := fmt.Sprintf("@lanovod-%d-%d", os.Getpid(), n)

	conn, err := net.DialUnix("unixgram",
		&net.UnixAddr{Name: local, Net: "unixgram"},
		&net.UnixAddr{Name: filepath.Join(SocketDir, Interface), Net: "unixgram"})
	if err != nil {
		return nil, fmt.Errorf("wpa_supplicant control socket: %w", err)
	}
	return &Control{conn: conn}, nil
}

func (c *Control) Close() error { return c.conn.Close() }

// A datagram reply that does not fit is truncated silently.
const reply = 32 << 10

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

// RECONNECT only acts after DISCONNECT, and answers OK either way.
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

func AwaitSupplicant() error {
	if c, err := Dial(); err == nil {
		_ = c.Close()
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
	_ = c.Close()
	return nil
}
