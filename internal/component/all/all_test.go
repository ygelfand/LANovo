package all

import (
	"path/filepath"
	"reflect"
	"testing"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
)

// Importing this package is what makes the registry complete, so it is also the only place the
// whole entity list can be looked at.
func entities(t *testing.T) []esphome.Entity {
	t.Helper()

	config.Use(filepath.Join(t.TempDir(), "state.json"))
	return component.Default().Entities()
}

// on is the sub-device an entity joined, read off Base rather than through a type switch: a switch
// would need a case per domain and would quietly skip whichever one was added next.
func on(e esphome.Entity) (uint32, bool) {
	v := reflect.Indirect(reflect.ValueOf(e)).FieldByName("Base").FieldByName("DeviceID")
	if !v.IsValid() || !v.CanUint() {
		return 0, false
	}
	return uint32(v.Uint()), true
}

// Every test here is over the whole entity list, so an empty one would pass all of them while
// proving nothing. This is what says the registry really did build the device.
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

// Every entity that joined a sub-device has to join one the device advertises. Home Assistant
// keys a sub-device on the address and the id, so an entity pointing at an id that is not in
// Info.Devices has nowhere to land.
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

// And the other way: a sub-device with nothing on it is a page in the registry with nothing to
// click into, which is worse than not having one.
func TestEverySubDeviceHasSomethingOnIt(t *testing.T) {
	count := map[uint32]int{}
	for _, e := range entities(t) {
		if id, ok := on(e); ok {
			count[id]++
		}
	}

	for _, d := range component.SubDevices() {
		if count[d.ID] == 0 {
			t.Errorf("%s (%d) is advertised with no entities on it", d.Name, d.ID)
		}
	}
}

// The numbers are identity: Home Assistant keys the registry entry on the address and the id, so
// two groups sharing one would merge and renumbering orphans what the old number named.
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

// Home Assistant's voice traffic has no entity to arrive through, so it goes to whichever component
// holds the satellite. Two of them holding one is not a conflict anything reports: both are
// registered, both answer, and which one Home Assistant ends up configuring depends on the order of
// the handler chain — so a wake word chosen in the interface could be set on the satellite that is
// not the one running the turns. Detection held a satellite of its own before the conversation
// existed, which is exactly the state this is here to stop coming back.
func TestOnlyOneComponentHoldsTheVoiceSatellite(t *testing.T) {
	config.Use(filepath.Join(t.TempDir(), "state.json"))

	want := reflect.TypeOf(&esphome.VoiceSatellite{})

	var who []string
	for _, c := range component.Default().All() {
		at := reflect.Indirect(reflect.ValueOf(c))
		if at.Kind() != reflect.Struct {
			continue
		}
		for i := range at.NumField() {
			if at.Type().Field(i).Type == want {
				who = append(who, c.Name())
			}
		}
	}

	if len(who) != 1 {
		t.Errorf("%d components hold a voice satellite: %v", len(who), who)
	}
}
