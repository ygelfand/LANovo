// Package diag is what the device says about itself: how hot it is, what it is running on, where
// on the network it is. Nothing here changes what it does.
package diag

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/metrics"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
)

func init() {
	component.Register(component.Network, Get, component.Order(90))
}

// The thermal zones worth showing. This board has about fifty, most of them per-core and per-step
// throttling zones that answer no question anybody asks.
//
// deca-cpu-max-step rather than one core: it is the part's own answer for how hot the hottest is,
// which is what throttling follows. On an idle device it read 34.8 where apc0-cpu0-usr read 31.6.
const (
	cpuZone = "deca-cpu-max-step"
	gpuZone = "gpu0-usr"
)

type Diag struct {
	temperature *esphome.Sensor
	gpuTemp     *esphome.Sensor
	cores       *esphome.Sensor
	coresOnline *esphome.Sensor
	usage       *esphome.Sensor
	load        *esphome.Sensor
	memory      *esphome.Sensor
	free        *esphome.Sensor
	uptime      *esphome.Sensor

	signal *esphome.Sensor
	rxRate *esphome.Sensor
	txRate *esphome.Sensor

	address *esphome.TextSensor
	network *esphome.TextSensor
	version *esphome.TextSensor

	interval *esphome.Number

	// wake restarts the collector's wait when the interval changes, so a shorter one takes effect
	// now rather than after the wait already running.
	wake chan struct{}

	last struct {
		at       time.Time
		rx, tx   float64
		recorded bool
	}

	// cpu is the jiffy counters from the previous sample. A percentage is the difference between
	// two readings, so the first one after a start reports nothing rather than averaging over
	// every second since boot.
	cpu struct {
		busy, total float64
		recorded    bool
	}
}

var (
	once   sync.Once
	shared *Diag
)

func Get() *Diag {
	once.Do(func() {
		shared = &Diag{wake: make(chan struct{}, 1)}
		shared.hardware()
		shared.radio()
		shared.identity()
		shared.collector()

		component.Subscribed.Listen(func(struct{}) { shared.soon() })
	})
	return shared
}

func (d *Diag) Name() string { return "diagnostics" }

func (d *Diag) Entities() []esphome.Entity {
	return []esphome.Entity{
		d.temperature, d.gpuTemp, d.cores, d.coresOnline, d.usage, d.load, d.memory, d.free, d.uptime,
		d.signal, d.rxRate, d.txRate,
		d.address, d.network, d.version,
		d.interval,
	}
}

func (d *Diag) Restore(c config.Config) { d.interval.Set(float32(c.Diag.Interval)) }

// hardware is what the machine is doing with itself.
func (d *Diag) hardware() {
	temp := func(id, name string) *esphome.Sensor {
		return &esphome.Sensor{
			Base: esphome.Base{
				ObjectID: id, Name: name, Icon: "mdi:thermometer",
				Category: esphome.CategoryDiagnostic,
			},
			Unit:        "°C",
			DeviceClass: "temperature",
			StateClass:  esphome.StateClassMeasurement,
			Decimals:    1,
		}
	}
	d.temperature = temp("cpu_temperature", "CPU temperature")
	d.gpuTemp = temp("gpu_temperature", "GPU temperature")

	count := func(id, name string) *esphome.Sensor {
		return &esphome.Sensor{
			Base: esphome.Base{
				ObjectID: id, Name: name, Icon: "mdi:cpu-64-bit",
				Category: esphome.CategoryDiagnostic,
			},
			StateClass: esphome.StateClassMeasurement,
		}
	}
	d.cores = count("cpu_cores", "CPU cores")
	d.coresOnline = count("cpu_cores_online", "CPU cores online")

	d.usage = &esphome.Sensor{
		Base: esphome.Base{
			ObjectID: "cpu_usage", Name: "CPU usage", Icon: "mdi:chip",
			Category: esphome.CategoryDiagnostic,
		},
		Unit:       "%",
		StateClass: esphome.StateClassMeasurement,
		Decimals:   1,
	}

	// Beside the usage rather than replaced by it. The load average is a true reading and Home
	// Assistant has been recording it; what it is not is an answer to how busy the board is,
	// because three kernel threads hold it at three however idle the device gets.
	d.load = &esphome.Sensor{
		Base: esphome.Base{
			ObjectID: "load_average", Name: "Load average", Icon: "mdi:gauge",
			Category: esphome.CategoryDiagnostic,
		},
		StateClass: esphome.StateClassMeasurement,
		Decimals:   2,
	}

	d.memory = &esphome.Sensor{
		Base: esphome.Base{
			ObjectID: "memory_free", Name: "Memory free", Icon: "mdi:memory",
			Category: esphome.CategoryDiagnostic,
		},
		Unit:       "MB",
		StateClass: esphome.StateClassMeasurement,
	}

	d.free = &esphome.Sensor{
		Base: esphome.Base{
			ObjectID: "disk_free", Name: "Disk free", Icon: "mdi:harddisk",
			Category: esphome.CategoryDiagnostic,
		},
		Unit:       "MB",
		StateClass: esphome.StateClassMeasurement,
	}

	d.uptime = &esphome.Sensor{
		Base: esphome.Base{
			ObjectID: "uptime", Name: "Uptime", Icon: "mdi:timer-outline",
			Category: esphome.CategoryDiagnostic,
		},
		Unit:        "s",
		DeviceClass: "duration",
		StateClass:  esphome.StateClassTotalIncreasing,
	}
}

// radio is the link and what is moving over it.
func (d *Diag) radio() {
	d.signal = &esphome.Sensor{
		Base: esphome.Base{
			ObjectID: "wifi_signal", Name: "Wi-Fi signal", Icon: "mdi:wifi",
			Category: esphome.CategoryDiagnostic,
		},
		Unit:        "dBm",
		DeviceClass: "signal_strength",
		StateClass:  esphome.StateClassMeasurement,
	}

	rate := func(id, name, icon string) *esphome.Sensor {
		return &esphome.Sensor{
			Base: esphome.Base{
				ObjectID: id, Name: name, Icon: icon, Category: esphome.CategoryDiagnostic,
			},
			Unit:       "kB/s",
			StateClass: esphome.StateClassMeasurement,
			Decimals:   1,
		}
	}
	d.rxRate = rate("wifi_rx_rate", "Wi-Fi received", "mdi:download")
	d.txRate = rate("wifi_tx_rate", "Wi-Fi sent", "mdi:upload")
}

// identity is where the device is, for finding it again.
func (d *Diag) identity() {
	d.address = &esphome.TextSensor{
		Base: esphome.Base{
			ObjectID: "ip_address", Name: "IP address", Icon: "mdi:ip-network",
			Category: esphome.CategoryDiagnostic,
		},
	}
	d.network = &esphome.TextSensor{
		Base: esphome.Base{
			ObjectID: "wifi_network", Name: "Wi-Fi network", Icon: "mdi:access-point-network",
			Category: esphome.CategoryDiagnostic,
		},
	}

	d.version = &esphome.TextSensor{
		Base: esphome.Base{
			ObjectID: "version", Name: "Version", Icon: "mdi:tag-outline",
			Category: esphome.CategoryDiagnostic,
		},
	}
	d.version.Set(layout.VersionString())
}

// collector builds the one setting these readings have: how often to take them.
func (d *Diag) collector() {
	d.interval = &esphome.Number{
		Base: esphome.Base{
			ObjectID: "metrics_interval",
			Name:     "Metrics interval",
			Icon:     "mdi:timer-sync",
			Category: esphome.CategoryConfig,
		},
		Min: 10, Max: 3600, Step: 10, Unit: "s",
		Mode: esphome.NumberBox,
	}

	d.interval.OnCommand = func(v float32) {
		d.interval.Set(v)
		if err := config.Set().Diag().Interval(int(v)); err != nil {
			slog.Error("saving the metrics interval failed", "err", err)
		}

		// The wait already running was measured against the old interval.
		d.soon()
	}
}

// soon asks the collector to sample without waiting out the rest of its interval.
func (d *Diag) soon() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// Run collects the readings that drift, once at the start and then on the interval.
//
// Immediately, not after the first wait: the readings go into entities that hold their last value
// until Home Assistant asks, so a device that waited would report nothing at all until then — and
// a restart is exactly when somebody is looking.
func (d *Diag) Run(ctx context.Context) error {
	for {
		d.Sample()

		t := time.NewTimer(time.Duration(config.Get().Diag.Interval) * time.Second)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-d.wake:
			t.Stop()
		case <-t.C:
		}
	}
}

// Sample takes every reading once.
func (d *Diag) Sample() {
	d.measure()
	d.wireless()
	d.address.Set(strings.Join(addresses(), ", "))
	d.network.Set(wifi.Get().Network())
}

func (d *Diag) measure() {
	r := metrics.Reader{}

	zones := r.Temperatures()
	if v, ok := zones[cpuZone]; ok {
		d.temperature.Set(float32(v))
	}
	if v, ok := zones[gpuZone]; ok {
		d.gpuTemp.Set(float32(v))
	}

	present, online := r.Cores()
	set(d.cores, present)
	set(d.coresOnline, online)

	d.busy(r)

	one, _ := r.Load()
	set(d.load, one)

	available, _ := r.Memory()
	if available.Known {
		d.memory.Set(float32(available.Value / 1024))
	}

	if free, err := metrics.Free(layout.StateDir); err == nil {
		d.free.Set(float32(free / (1024 * 1024)))
	}

	d.uptime.Set(float32(metrics.Uptime()))
}

// busy publishes how much of the time since the last sample the processor spent working.
//
// From the difference between two readings of /proc/stat, so it is the interval that is being
// described rather than the whole time since boot. The first sample after a start has nothing to
// subtract and reports nothing.
func (d *Diag) busy(r metrics.Reader) {
	busy, total := r.CPU()
	if !busy.Known || !total.Known {
		return
	}

	was := d.cpu
	d.cpu.busy, d.cpu.total, d.cpu.recorded = busy.Value, total.Value, true

	if !was.recorded {
		return
	}

	// A counter that went backwards is a machine that rebooted under us, or a file read that came
	// back torn. Either way there is nothing to report until the next pair.
	elapsed := total.Value - was.total
	worked := busy.Value - was.busy
	if elapsed <= 0 || worked < 0 {
		return
	}
	d.usage.Set(float32(min(worked/elapsed, 1) * 100))
}

// wireless publishes the link and what is moving over it. Rates come from what changed since the
// last sample, so the first one after a start reports nothing rather than counting from boot.
func (d *Diag) wireless() {
	signal, rx, tx := metrics.Reader{}.Wifi()
	set(d.signal, signal)

	now := time.Now()
	was := d.last
	d.last.at, d.last.rx, d.last.tx, d.last.recorded = now, rx.Value, tx.Value, true

	if !was.recorded {
		return
	}
	secs := now.Sub(was.at).Seconds()
	if secs <= 0 {
		return
	}

	if rx.Known {
		d.rxRate.Set(float32((rx.Value - was.rx) / secs / 1024))
	}
	if tx.Known {
		d.txRate.Set(float32((tx.Value - was.tx) / secs / 1024))
	}
}

func addresses() []string {
	ips := metrics.Addresses()
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	return out
}

// set publishes a reading, leaving the entity at its last value when the file was not there. A
// sensor that reports zero for absent reads like a measurement.
func set(s *esphome.Sensor, r metrics.Reading) {
	if r.Known {
		s.Set(float32(r.Value))
	}
}
