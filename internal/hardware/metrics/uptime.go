package metrics

import (
	"os"
	"strconv"
	"strings"
	"time"
)

var bootedAt, readAt = readUptime()

func readUptime() (float64, time.Time) {
	b, err := os.ReadFile("/proc/uptime")
	now := time.Now()
	if err != nil {
		return 0, now
	}
	first, _, _ := strings.Cut(string(b), " ")
	v, _ := strconv.ParseFloat(first, 64)
	return v, now
}

func Uptime() float64 { return bootedAt + time.Since(readAt).Seconds() }
