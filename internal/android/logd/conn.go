package logd

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"
)

const socket = "/dev/socket/logdw"

// LOG_ID_MAIN, as liblog uses it.
const idMain = 0

type severity struct {
	Level    slog.Level
	Priority byte
	Letter   string
}

var severities = []severity{
	{slog.LevelError, 6, "E"},
	{slog.LevelWarn, 5, "W"},
	{slog.LevelInfo, 4, "I"},
	{slog.LevelDebug, 3, "D"},
}

func rung(l slog.Level) severity {
	for _, s := range severities {
		if l >= s.Level {
			return s
		}
	}
	return severities[len(severities)-1]
}

func Priority(l slog.Level) byte { return rung(l).Priority }

func Letter(l slog.Level) string { return rung(l).Letter }

func Level(prio byte) slog.Level {
	for _, s := range severities {
		if prio >= s.Priority {
			return s.Level
		}
	}
	return slog.LevelDebug
}

type conn struct {
	sock net.Conn
	tag  string
}

func dial(tag string) (*conn, error) {
	c, err := net.Dial("unixgram", socket)
	if err != nil {
		return nil, fmt.Errorf("logd: dial %s: %w", socket, err)
	}
	return &conn{sock: c, tag: tag}, nil
}

func (c *conn) Close() error { return c.sock.Close() }

// Wire format: buffer id, thread id, realtime, then priority, NUL-terminated tag and message.
func (c *conn) write(prio byte, msg string) error {
	now := time.Now()

	buf := make([]byte, 0, 11+1+len(c.tag)+1+len(msg)+1)
	buf = append(buf, idMain)
	buf = binary.LittleEndian.AppendUint16(buf, uint16(os.Getpid()))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(now.Unix()))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(now.Nanosecond()))
	buf = append(buf, prio)
	buf = append(buf, c.tag...)
	buf = append(buf, 0)
	buf = append(buf, msg...)
	buf = append(buf, 0)

	_, err := c.sock.Write(buf)
	return err
}
