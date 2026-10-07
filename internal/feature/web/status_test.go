package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/metrics"
)

// part is a component with nothing but an opinion about coming up.
type part struct {
	name string
	p    component.Progress
}

func (f part) Name() string                { return f.name }
func (f part) Startup() component.Progress { return f.p }

func registered(f part) func() component.Component {
	return func() component.Component { return f }
}

// The root is the status page, and since the onboarding page went it is the only page there is.
func TestTheRootIsTheStatusPage(t *testing.T) {
	rec := httptest.NewRecorder()
	Get().status(rec, httptest.NewRequest(http.MethodGet, "http://10.0.0.5/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "<!doctype html>") {
		t.Error("the root did not render a page")
	}
}

func TestStatusRefusesAnythingElse(t *testing.T) {
	rec := httptest.NewRecorder()
	Get().status(rec, httptest.NewRequest(http.MethodGet, "http://10.0.0.5/nonesuch", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("an unknown path gave %d, want 404", rec.Code)
	}
}

// The registry is walked in the order the device starts things, and each component's own answer
// is what the row says. A component that failed is not the same as one still waiting: the device
// carries on without the first and is held up by the second.
func TestComponentsReportWhatTheySayTheyAre(t *testing.T) {
	reg := component.New()
	reg.Add(component.Hardware, registered(part{"panel", component.Progress{Done: true}}))
	reg.Add(component.Hardware, registered(part{"wifi", component.Progress{Doing: "associating"}}))
	reg.Add(
		component.Network,
		registered(part{"api", component.Progress{Failed: true, Doing: "no key"}}),
	)

	got := parts(reg)
	if len(got) != 3 {
		t.Fatalf("got %d parts, want 3", len(got))
	}

	want := []Part{
		{"panel", "ready", ""},
		{"wifi", "waiting", "associating"},
		{"api", "failed", "no key"},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("part %d is %+v, want %+v", i, got[i], w)
		}
	}
}

// A component with nothing to say about coming up is not on the list, rather than on it as a row
// that never resolves.
func TestAComponentWithNoProgressIsNotListed(t *testing.T) {
	reg := component.New()
	reg.Add(component.Device, func() component.Component { return quiet{} })

	if got := parts(reg); len(got) != 0 {
		t.Errorf("got %+v, want nothing", got)
	}
}

type quiet struct{}

func (quiet) Name() string { return "quiet" }

// Nothing on this page is a secret. It is served unauthenticated to anyone who can reach port 80,
// and the process it is rendered from holds the encryption key and the network's passphrase.
func TestTheStatusPageCarriesNoSecrets(t *testing.T) {
	for _, field := range reflect.VisibleFields(reflect.TypeFor[Status]()) {
		switch strings.ToLower(field.Name) {
		case "key", "secret", "password", "passphrase", "token", "psk":
			t.Errorf("Status carries a %s", field.Name)
		}
	}

	// And the template asks for nothing the struct does not have to offer.
	for _, banned := range []string{".Key", ".Secret", ".Password", ".Token", ".PSK"} {
		if strings.Contains(statusHTML, banned) {
			t.Errorf("the template renders %s", banned)
		}
	}
}

// The zone set on the device outranks the one Home Assistant sends, and the page says which is in
// force: a clock an hour out is otherwise a mystery with two plausible causes.
func TestTheZoneSaysWhereItCameFrom(t *testing.T) {
	tests := []struct {
		name           string
		chosen, home   string
		wantZone, want string
	}{
		{
			"chosen wins",
			"Europe/Berlin",
			"EST5EDT,M3.2.0,M11.1.0",
			"Europe/Berlin",
			"Set on the device",
		},
		{
			"home otherwise",
			"",
			"EST5EDT,M3.2.0,M11.1.0",
			"EST5EDT,M3.2.0,M11.1.0",
			"From Home Assistant",
		},
		{"neither", "", "", unknown, "Nothing has said"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c config.Config
			c.Time.Chosen, c.Time.Home = tt.chosen, tt.home

			rows := rowsOf(clock(c))
			if got := rows["Zone"]; got != tt.wantZone {
				t.Errorf("zone = %q, want %q", got, tt.wantZone)
			}
			if got := rows["Source"]; got != tt.want {
				t.Errorf("source = %q, want %q", got, tt.want)
			}
		})
	}
}

// Two of the forty nine zones the kernel exposes, named for what they are. The rest are per-core
// throttling steps and would bury the page.
func TestOnlyTheTwoTemperaturesWorthShowing(t *testing.T) {
	root := t.TempDir()
	zone(t, root, 0, "apc0-cpu0-usr", "33400")
	zone(t, root, 1, "deca-cpu-max-step", "35500")
	zone(t, root, 2, "gpu0-usr", "31900")

	got := hottest(metrics.Reader{Root: root})
	want := []Row{{"CPU", "35.5 °C"}, {"GPU", "31.9 °C"}}

	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d is %+v, want %+v", i, got[i], want[i])
		}
	}
}

// meminfo counts kilobytes. Read as bytes it turned a 1.9 GiB board into a 1.9 MiB one, which is
// the sort of figure somebody acts on.
func TestMemoryIsReportedFromKilobytes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "proc"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "proc/meminfo"),
		"MemTotal:        2027056 kB\nMemAvailable:     1648992 kB")

	got := rowsOf(system(metrics.Reader{Root: root}))["Memory free"]
	if want := "1.6 GiB of 1.9 GiB"; got != want {
		t.Errorf("memory = %q, want %q", got, want)
	}
}

// A board with no thermal zones is a board with no temperature rows, not rows reading zero.
func TestNoThermalZonesMeansNoRows(t *testing.T) {
	if got := hottest(metrics.Reader{Root: t.TempDir()}); got != nil {
		t.Errorf("got %+v, want nothing", got)
	}
}

func zone(t *testing.T, root string, n int, name, milli string) {
	t.Helper()

	dir := filepath.Join(root, "sys/class/thermal/thermal_zone"+strconv.Itoa(n))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "type"), name)
	write(t, filepath.Join(dir, "temp"), milli)
}

func write(t *testing.T, path, s string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(s+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUptimeReadsAtEverySize(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{7, "7s"},
		{95, "1m 35s"},
		{3700, "1h 1m"},
		{100000, "1d 3h"},
		{1000000, "11d 13h"},
	}

	for _, tt := range tests {
		if got := since(tt.seconds); got != tt.want {
			t.Errorf("since(%v) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestSizesPickTheUnitTheyFill(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{512, "512 B"},
		{2048, "2.0 KiB"},
		{5 * 1024 * 1024, "5.0 MiB"},
		{3 * 1024 * 1024 * 1024, "3.0 GiB"},
	}

	for _, tt := range tests {
		if got := bytes(tt.v); got != tt.want {
			t.Errorf("bytes(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestOffsetsReadAsUTC(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{0, "UTC+00:00"},
		{3600, "UTC+01:00"},
		{-18000, "UTC-05:00"},
		{19800, "UTC+05:30"},
		{-34200, "UTC-09:30"},
	}

	for _, tt := range tests {
		if got := utc(tt.seconds); got != tt.want {
			t.Errorf("utc(%d) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

// A reading the board did not give says so, rather than leaving a gap that reads as a value.
func TestAMissingReadingSaysSo(t *testing.T) {
	if got := or("   "); got != unknown {
		t.Errorf("or(blank) = %q, want %q", got, unknown)
	}
	if got := or("kitchen"); got != "kitchen" {
		t.Errorf("or(%q) = %q, want it unchanged", "kitchen", got)
	}
}

// rowsOf is a group as a map, for a test that cares about one row and not where it sits.
func rowsOf(g Group) map[string]string {
	out := map[string]string{}
	for _, r := range g.Rows {
		out[r.Name] = r.Value
	}
	return out
}
