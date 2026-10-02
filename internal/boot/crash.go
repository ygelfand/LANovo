package boot

import (
	"bufio"
	"bytes"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
)

func keepCrashes(path string) {
	last := path + ".last"
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		if err := os.Rename(path, last); err != nil {
			slog.Error("keeping the last crash report failed", "err", err)
		}
		slog.Error("lanovod died on the last run", "fatal", fatalLine(b), "report", last)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		slog.Error("opening the crash report failed", "path", path, "err", err)
		return
	}
	defer f.Close()
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		slog.Error("pointing the runtime at the crash report failed", "err", err)
	}
}

func fatalLine(report []byte) string {
	first := ""
	lines := bufio.NewScanner(bytes.NewReader(report))
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if strings.HasPrefix(line, "fatal error:") || strings.HasPrefix(line, "panic:") {
			return line
		}
		if first == "" {
			first = line
		}
	}
	return first
}
