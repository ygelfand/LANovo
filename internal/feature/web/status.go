package web

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/metrics"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
)

//go:embed status.html
var statusHTML string

var statusPage = template.Must(template.New("status").Parse(statusHTML))

// Status is what the device says about itself: what it is, what came up, and what it is set to.
//
// Nothing here is a secret. The encryption key, the network's passphrase and the onboarding code
// are all reachable from this process and none of them appear: the page is unauthenticated to
// anyone who can open port 80.
type Status struct {
	Name    string
	Model   string
	Version string

	// Adopted is whether Home Assistant has ever subscribed, which is the one thing on the page
	// somebody might be waiting for.
	Adopted bool

	Parts  []Part
	Groups []Group
}

// Part is one component and how far it got, in the order the device starts them.
type Part struct {
	Name string

	// State is ready, failed or waiting, and is the class the row is styled by as well as what it
	// says.
	State string

	// Doing is what it is waiting on, or why it failed. Empty once it is up.
	Doing string
}

// Group is one card: a heading and the rows under it.
type Group struct {
	Title string
	Rows  []Row
}

type Row struct{ Name, Value string }

// unknown is what a row shows for a reading the board did not give. Written out rather than left
// blank, so a missing sensor reads as missing instead of as an empty page.
const unknown = "—"

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	statusPage.Execute(w, statusOf(component.Default(), metrics.Reader{}))
}

// statusOf gathers what the page shows.
//
// The registry and the reader are passed in rather than reached for, so a test can ask a registry
// of its own and a directory of fixtures instead of building the real components and reading this
// machine's sysfs.
func statusOf(reg *component.Registry, r metrics.Reader) Status {
	c := config.Get()

	return Status{
		Name:    c.Device.Name,
		Model:   c.Device.Model,
		Version: layout.VersionString(),
		Adopted: Adopted(),
		Parts:   parts(reg),
		Groups: []Group{
			network(r),
			clock(c),
			screen(c),
			volumes(c),
			system(r),
		},
	}
}

// parts is every component that says something about coming up.
func parts(reg *component.Registry) []Part {
	progress := reg.Progress()

	out := make([]Part, 0, len(progress))
	for _, p := range progress {
		out = append(out, Part{Name: p.Name, State: state(p), Doing: p.Doing})
	}
	return out
}

func state(p component.Progress) string {
	switch {
	case p.Failed:
		return "failed"
	case p.Done:
		return "ready"
	}
	return "waiting"
}

func network(r metrics.Reader) Group {
	rows := []Row{
		{"Wi-Fi network", or(wifi.Get().Network())},
		{"MAC", or(wifi.Get().MAC())},
	}

	// Every address, v6 included. This is a page for looking at rather than a code to scan, so
	// the reason the onboarding screen shows only v4 does not apply.
	ips := metrics.Addresses()
	if len(ips) == 0 {
		rows = append(rows, Row{"Address", unknown})
	}
	for i, ip := range ips {
		name := "Address"
		if i > 0 {
			name = ""
		}
		rows = append(rows, Row{name, ip.String()})
	}

	if signal, _, _ := r.Wifi(); signal.Known {
		rows = append(rows, Row{"Signal", fmt.Sprintf("%.0f dBm", signal.Value)})
	}
	return Group{Title: "Network", Rows: rows}
}

// clock is the time as the device tells it, and where the zone came from. Which of the two zones
// is in force is worth saying: one set on the device outranks the server, and a clock an hour out
// is otherwise a mystery.
func clock(c config.Config) Group {
	now := time.Now()

	zone, from := c.Time.Chosen, "Set on the device"
	if zone == "" {
		zone, from = c.Time.Home, "From Home Assistant"
	}
	if zone == "" {
		zone, from = unknown, "Nothing has said"
	}

	name, offset := now.Zone()
	return Group{Title: "Time", Rows: []Row{
		{"Local time", now.Format("15:04:05")},
		{"Date", now.Format("Monday, 2 January 2006")},
		{"Zone", zone},
		{"Source", from},
		{"Running as", fmt.Sprintf("%s (%s)", name, utc(offset))},
	}}
}

// utc is an offset in seconds as the hours and minutes anyone reads it in.
func utc(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign, seconds = "-", -seconds
	}
	return fmt.Sprintf("UTC%s%02d:%02d", sign, seconds/3600, seconds%3600/60)
}

func screen(c config.Config) Group {
	return Group{Title: "Screen", Rows: []Row{
		{"Theme", c.Screen.Theme},
		{"Brightness", c.Screen.Mode.Label()},
		{"Backlight", percent(c.Screen.Backlight)},
		{"Clock", c.Screen.Hours.Label()},
		{"Drawer", c.Screen.Drawer.Label()},
		{"Logo", yes(c.Screen.Logo)},
	}}
}

func volumes(c config.Config) Group {
	rows := make([]Row, 0, 4)
	for _, s := range []config.Stream{
		config.StreamMedia, config.StreamAlerts, config.StreamVoice, config.StreamFeedback,
	} {
		rows = append(rows, Row{s.Label(), percent(c.Volume.Level(s))})
	}
	return Group{Title: "Volume", Rows: rows}
}

// system is the machine underneath: how long it has been up, how loaded it is and how hot.
func system(r metrics.Reader) Group {
	rows := []Row{{"Uptime", since(metrics.Uptime())}}

	// Since boot, not since a moment ago: this page is rendered once per request and has no
	// previous reading to subtract. It answers "what has this device been doing", where the
	// sensor in Home Assistant answers "what is it doing now".
	if busy, total := r.CPU(); busy.Known && total.Known && total.Value > 0 {
		rows = append(rows, Row{"CPU since boot", fmt.Sprintf("%.1f%% busy", busy.Value/total.Value*100)})
	}

	// The floor is said out loud. Three kernel threads sit permanently in uninterruptible sleep on
	// this board and Linux counts those, so the load never reads below three and a number that
	// does not say so reads as a device in trouble.
	if one, five := r.Load(); one.Known {
		rows = append(rows, Row{"Load", fmt.Sprintf("%.2f, %.2f (floor ~3)", one.Value, five.Value)})
	}
	if _, online := r.Cores(); online.Known {
		rows = append(rows, Row{"Cores online", strconv.Itoa(int(online.Value))})
	}
	// meminfo is in kB, which is the one reading on this page that is not already a byte count.
	if free, total := r.Memory(); free.Known && total.Known {
		rows = append(rows, Row{"Memory free", fmt.Sprintf("%s of %s", bytes(free.Value*1024), bytes(total.Value*1024))})
	}
	if free, err := metrics.Free(layout.StateDir); err == nil {
		rows = append(rows, Row{"Disk free", bytes(float64(free))})
	}
	return Group{Title: "System", Rows: append(rows, hottest(r)...)}
}

// hottest is the two zones worth showing of the forty nine the kernel exposes: the part's own
// answer for the hottest core, and the GPU. The rest are per-core throttling steps.
func hottest(r metrics.Reader) []Row {
	zones := r.Temperatures()

	var out []Row
	for _, z := range []struct{ name, zone string }{
		{"CPU", "deca-cpu-max-step"},
		{"GPU", "gpu0-usr"},
	} {
		if v, ok := zones[z.zone]; ok {
			out = append(out, Row{z.name, fmt.Sprintf("%.1f °C", v)})
		}
	}
	return out
}

// since is a duration in seconds, to the two units that matter at that size. A device that has
// been up eleven days does not need the seconds.
func since(seconds float64) string {
	d := time.Duration(seconds) * time.Second

	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// bytes is a size in the largest unit it fills.
func bytes(v float64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}

	i := 0
	for v >= 1024 && i < len(units)-1 {
		v, i = v/1024, i+1
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func percent(v int) string { return strconv.Itoa(v) + "%" }

func yes(v bool) string {
	if v {
		return "On"
	}
	return "Off"
}

// or fills in for a reading that is not there yet, which on this page is most of them for the
// first few seconds after boot.
func or(s string) string {
	if strings.TrimSpace(s) == "" {
		return unknown
	}
	return s
}
