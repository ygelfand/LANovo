package all

import (
	"path/filepath"
	"reflect"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
)

func entities(t *testing.T) []esphome.Entity {
	t.Helper()

	config.Use(filepath.Join(t.TempDir(), "state.json"))
	return component.Default().Entities()
}

func on(e esphome.Entity) (uint32, bool) {
	v := reflect.Indirect(reflect.ValueOf(e)).FieldByName("Base").FieldByName("DeviceID")
	if !v.IsValid() || !v.CanUint() {
		return 0, false
	}
	return uint32(v.Uint()), true
}

func TestTheRegistryBuildsTheWholeDevice(t *testing.T) {
	got := entities(t)

	if len(got) < 20 {
		t.Fatalf("the registry built %d entities, too few to be this device", len(got))
	}

	count := map[uint32]int{}
	for _, e := range got {
		id, _ := on(e)
		count[id]++
	}
	t.Logf("%d entities: %d on the device itself, %v on sub-devices", len(got), count[0], count)
}

func TestEveryEntityLandsOnASubDeviceThatExists(t *testing.T) {
	known := map[uint32]bool{0: true}
	for _, d := range component.SubDevices() {
		known[d.ID] = true
	}

	for _, e := range entities(t) {
		id, ok := on(e)
		if !ok {
			t.Errorf("%s has no Base.DeviceID to read", e.Object())
			continue
		}
		if !known[id] {
			t.Errorf("%s is on sub-device %d, which is not advertised", e.Object(), id)
		}
	}
}

func TestEverySubDeviceHasSomethingOnIt(t *testing.T) {
	count := map[uint32]int{}
	for _, e := range entities(t) {
		if id, ok := on(e); ok {
			count[id]++
		}
	}

	for _, d := range component.SubDevices() {
		if count[d.ID] == 0 && !component.Sometimes[d.ID] {
			t.Errorf("%s (%d) is advertised with no entities on it", d.Name, d.ID)
		}
	}
}

func TestSubDeviceIDsAreDistinctAndNotTheDeviceItself(t *testing.T) {
	seen := map[uint32]bool{}

	for _, d := range component.SubDevices() {
		if d.ID == 0 {
			t.Errorf("%s is numbered zero, which is the device itself", d.Name)
		}
		if seen[d.ID] {
			t.Errorf("two sub-devices are numbered %d", d.ID)
		}
		seen[d.ID] = true
	}
}

func TestOnlyOneComponentHoldsTheVoiceSatellite(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	want := reflect.TypeOf(&esphome.VoiceSatellite{})

	var who []string
	for _, c := range component.Default().All() {
		if holds(reflect.ValueOf(c), want) {
			who = append(who, c.Name())
		}
	}

	if len(who) != 1 {
		t.Errorf("%d components hold a voice satellite: %v", len(who), who)
	}
}

func holds(v reflect.Value, want reflect.Type) bool {
	at := reflect.Indirect(v)
	if at.Kind() != reflect.Struct {
		return false
	}
	for i := range at.NumField() {
		f := at.Type().Field(i)
		if f.Type == want {
			return true
		}
		if f.Anonymous && holds(at.Field(i), want) {
			return true
		}
	}
	return false
}
