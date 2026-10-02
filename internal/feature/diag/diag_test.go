package diag

import (
	"path/filepath"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/metrics"
)

func fresh(t *testing.T) *Diag {
	t.Helper()

	config.Use(filepath.Join(t.TempDir(), "state.json"))
	return Get()
}

// Every entity has to be built, or the server refuses the lot when one is nil.
func TestEveryEntityExists(t *testing.T) {
	for i, e := range fresh(t).Entities() {
		if e == nil {
			t.Errorf("entity %d is nil", i)
		}
	}
}

// Two entities with one object id is a collision Home Assistant resolves by dropping one.
func TestObjectIDsAreDistinct(t *testing.T) {
	seen := map[string]bool{}

	for _, e := range fresh(t).Entities() {
		id := objectID(e)
		if id == "" {
			t.Errorf("%T has no object id", e)
			continue
		}
		if seen[id] {
			t.Errorf("two entities are called %q", id)
		}
		seen[id] = true
	}
}

func objectID(e esphome.Entity) string {
	switch v := e.(type) {
	case *esphome.Sensor:
		return v.ObjectID
	case *esphome.TextSensor:
		return v.ObjectID
	case *esphome.Number:
		return v.ObjectID
	}
	return ""
}

func TestRestoreTakesTheSavedInterval(t *testing.T) {
	d := fresh(t)

	cfg := config.Defaults()
	cfg.Diag.Interval = 120
	d.Restore(cfg)

	if got := d.interval.Get(); got != 120 {
		t.Errorf("interval = %v, want 120", got)
	}
}

// A reading the device could not take leaves the entity at its last value: a sensor that reports
// zero for an absent file reads like a measurement.
func TestSetLeavesUnknownReadingsAlone(t *testing.T) {
	s := &esphome.Sensor{Base: esphome.Base{ObjectID: "test"}}
	s.Set(42)

	set(s, metrics.Reading{Value: 99, Known: false})

	if got := s.Get(); got != 42 {
		t.Errorf("an unknown reading overwrote the entity with %v", got)
	}

	set(s, metrics.Reading{Value: 7, Known: true})
	if got := s.Get(); got != 7 {
		t.Errorf("a known reading did not reach the entity: %v", got)
	}
}

// The zones this board actually has, so a rename in the kernel shows up here rather than as a
// sensor that silently stops reporting.
//
// The CPU one is the part's hottest step rather than a single core, which is what throttling
// follows and reads a few degrees above any one of them.
func TestThermalZonesAreThisBoards(t *testing.T) {
	if cpuZone != "deca-cpu-max-step" {
		t.Errorf("cpu zone is %q", cpuZone)
	}
	if gpuZone != "gpu0-usr" {
		t.Errorf("gpu zone is %q", gpuZone)
	}
}

// Sampling runs on a device with none of these files present, so it must not panic when every
// reading is missing.
func TestSampleOnAMachineWithNothing(t *testing.T) {
	d := fresh(t)
	d.Sample()
	d.Sample()
}
