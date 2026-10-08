package metrics

import (
	"os"
	"strconv"
	"strings"
)

type Reader struct {
	Root string
}

type Reading struct {
	Value float64
	Known bool
}

func known(v float64) Reading { return Reading{Value: v, Known: true} }

// The kernel reports millidegrees.
func (r Reader) Temperatures() map[string]float64 {
	out := map[string]float64{}
	for i := range zones {
		zone := r.path("sys/class/thermal/thermal_zone" + strconv.Itoa(i))

		name, err := text(zone + "/type")
		if err != nil {
			continue
		}
		milli, err := number(zone + "/temp")
		if err != nil {
			continue
		}
		out[name] = milli / 1000
	}
	return out
}

const zones = 64

func (r Reader) Cores() (present, online Reading) {
	return r.count("sys/devices/system/cpu/present"), r.count("sys/devices/system/cpu/online")
}

func (r Reader) Load() (one, five Reading) {
	line, err := text(r.path("proc/loadavg"))
	if err != nil {
		return Reading{}, Reading{}
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		return Reading{}, Reading{}
	}
	return parse(fields[0]), parse(fields[1])
}

func (r Reader) CPU() (busy, total Reading) {
	body, err := os.ReadFile(r.path("proc/stat"))
	if err != nil {
		return Reading{}, Reading{}
	}

	for line := range strings.SplitSeq(string(body), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}

		fields := strings.Fields(line)[1:]
		if len(fields) < 5 {
			return Reading{}, Reading{}
		}

		var all, idle float64
		for i, f := range fields {
			v, err := strconv.ParseFloat(f, 64)
			if err != nil {
				return Reading{}, Reading{}
			}
			all += v

			if i == 3 || i == 4 {
				idle += v
			}
		}
		return known(all - idle), known(all)
	}
	return Reading{}, Reading{}
}

func (r Reader) Memory() (available, total Reading) {
	body, err := os.ReadFile(r.path("proc/meminfo"))
	if err != nil {
		return Reading{}, Reading{}
	}

	for line := range strings.SplitSeq(string(body), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		kb := parse(strings.TrimSuffix(strings.TrimSpace(value), " kB"))
		switch key {
		case "MemAvailable":
			available = kb
		case "MemTotal":
			total = kb
		}
	}
	return available, total
}

// The kernel keeps the signal as an unsigned byte: above 127 is a negative dBm.
func (r Reader) Wifi() (signal, rx, tx Reading) {
	if body, err := os.ReadFile(r.path("proc/net/wireless")); err == nil {
		for line := range strings.SplitSeq(string(body), "\n") {
			name, rest, ok := strings.Cut(line, ":")
			if !ok || strings.TrimSpace(name) != wifiInterface {
				continue
			}
			if fields := strings.Fields(rest); len(fields) >= 3 {
				if level := parse(strings.TrimSuffix(fields[2], ".")); level.Known {
					if level.Value > 127 {
						level.Value -= 256
					}
					signal = level
				}
			}
		}
	}

	body, err := os.ReadFile(r.path("proc/net/dev"))
	if err != nil {
		return signal, Reading{}, Reading{}
	}

	for line := range strings.SplitSeq(string(body), "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) != wifiInterface {
			continue
		}
		if fields := strings.Fields(rest); len(fields) >= 9 {
			rx, tx = parse(fields[0]), parse(fields[8])
		}
	}
	return signal, rx, tx
}

const wifiInterface = "wlan0"

func (r Reader) count(path string) Reading {
	list, err := text(r.path(path))
	if err != nil {
		return Reading{}
	}

	var n float64
	for part := range strings.SplitSeq(list, ",") {
		lo, hi, ranged := strings.Cut(part, "-")
		first, err := strconv.Atoi(strings.TrimSpace(lo))
		if err != nil {
			continue
		}
		if !ranged {
			n++
			continue
		}
		last, err := strconv.Atoi(strings.TrimSpace(hi))
		if err != nil {
			continue
		}
		n += float64(last - first + 1)
	}
	if n == 0 {
		return Reading{}
	}
	return known(n)
}

func (r Reader) path(p string) string {
	if r.Root == "" {
		return "/" + p
	}
	return r.Root + "/" + p
}

func text(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func number(path string) (float64, error) {
	body, err := text(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(body, 64)
}

func parse(s string) Reading {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return Reading{}
	}
	return known(v)
}
