// Package metrics reads what the machine says about itself: how hot it is, how many cores are
// running, how much memory is left, and where on the network it can be found.
//
// The readings are files in sysfs or proc, so one costs nothing and a missing one is not an error — a
// board that does not have a sensor simply has no value for it.
package metrics

import (
	"os"
	"strconv"
	"strings"
)

// Paths are relative to a root so a test can point them at a directory of fixtures rather than at the
// machine running the test.
type Reader struct {
	Root string
}

// Reading is one number the board reported, and whether it reported one at all.
type Reading struct {
	Value float64
	Known bool
}

func known(v float64) Reading { return Reading{Value: v, Known: true} }

// Temperatures are the thermal zones by the name the kernel gives them, in degrees. This board has
// 49, most of them board sensors: apc0-cpu0-usr is the CPU and gpu0-usr the GPU.
//
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

// zones is how far the scan looks. The board has 49 and the ones worth reading are not first.
const zones = 64

// Cores is how many the kernel could use and how many are online. They differ: this kernel hotplugs
// cores, so a device with four can be running two, and any figure about CPU usage means something
// different depending on which it is.
func (r Reader) Cores() (present, online Reading) {
	return r.count("sys/devices/system/cpu/present"), r.count("sys/devices/system/cpu/online")
}

// Load is the one and five minute load averages.
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

// CPU is how much work the kernel has done since boot and how much time there was to do it in,
// both in jiffies. Two counters rather than a percentage: the answer anyone wants is how busy it
// has been lately, which is the difference between two readings.
//
// Worth having beside the load average, which on this board never reads below 3.00 however idle it
// is. Three kernel threads — mdss_dsi_event, mmc-cmdqd/0 and mdss_fb0 — sit permanently in
// uninterruptible sleep, and Linux counts those towards the load alongside anything runnable. The
// number is not wrong, it just has a floor, and nothing reading it can tell.
func (r Reader) CPU() (busy, total Reading) {
	body, err := os.ReadFile(r.path("proc/stat"))
	if err != nil {
		return Reading{}, Reading{}
	}

	for line := range strings.SplitSeq(string(body), "\n") {
		// The first line is every core together. The per core ones that follow start "cpu0" and
		// are not what this is for.
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

			// Idle and iowait: the processor had nothing to run either way.
			if i == 3 || i == 4 {
				idle += v
			}
		}
		return known(all - idle), known(all)
	}
	return Reading{}, Reading{}
}

// Memory is what the kernel says is available and what the board has, in kilobytes. Available rather
// than free: free counts only untouched pages and reads alarmingly low on a machine that is working
// properly.
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

// Wifi is what the radio reports: the signal in dBm, and the bytes carried since boot.
//
// The kernel keeps the signal as an unsigned byte, so anything above 127 is a negative dBm.
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

// wifiInterface is the only one on this board.
const wifiInterface = "wlan0"

// count reads a cpu list like "0-3" or "0-1,3" and reports how many it names.
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
