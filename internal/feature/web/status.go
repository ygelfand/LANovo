package web

import (
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	netaddress "github.com/ygelfand/libcountertop/pkg/network/address"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/metrics"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
)

//go:embed status.html
var statusHTML string

var statusPage = template.Must(template.New("status").Parse(statusHTML))

type Status struct {
	Name    string
	Model   string
	Version string

	Adopted bool

	Parts  []Part
	Groups []Group
}

type Part struct {
	Name string

	State string

	Doing string
}

type Group struct {
	Title string
	Rows  []Row
}

type Row struct{ Name, Value string }

const unknown = "—"

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	if err := statusPage.Execute(w, statusOf(component.Default(), metrics.Reader{})); err != nil {
		slog.Error("rendering the status page failed", "err", err)
	}
}

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

	ips := netaddress.Addresses()
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

func system(r metrics.Reader) Group {
	rows := []Row{{"Uptime", since(metrics.Uptime())}}

	if busy, total := r.CPU(); busy.Known && total.Known && total.Value > 0 {
		rows = append(
			rows,
			Row{"CPU since boot", fmt.Sprintf("%.1f%% busy", busy.Value/total.Value*100)},
		)
	}

	if one, five := r.Load(); one.Known {
		rows = append(
			rows,
			Row{"Load", fmt.Sprintf("%.2f, %.2f (floor ~3)", one.Value, five.Value)},
		)
	}
	if _, online := r.Cores(); online.Known {
		rows = append(rows, Row{"Cores online", strconv.Itoa(int(online.Value))})
	}
	// /proc/meminfo reports kB.
	if free, total := r.Memory(); free.Known && total.Known {
		rows = append(
			rows,
			Row{
				"Memory free",
				fmt.Sprintf("%s of %s", bytes(free.Value*1024), bytes(total.Value*1024)),
			},
		)
	}
	if free, err := metrics.Free(layout.StateDir); err == nil {
		rows = append(rows, Row{"Disk free", bytes(float64(free))})
	}
	return Group{Title: "System", Rows: append(rows, hottest(r)...)}
}

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

func or(s string) string {
	if strings.TrimSpace(s) == "" {
		return unknown
	}
	return s
}
