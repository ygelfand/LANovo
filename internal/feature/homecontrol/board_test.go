package homecontrol

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/states"
)

var lit = []homeassistant.Entity{
	{ID: "light.k1", Name: "Kitchen Center", State: "on", AreaID: "kitchen", Area: "Kitchen"},
	{ID: "light.k2", Name: "Kitchen Ring", State: "off", AreaID: "kitchen", Area: "Kitchen"},
	{ID: "light.k3", Name: "Kitchen Strip", State: "unavailable", AreaID: "kitchen", Area: "Kitchen"},
	{ID: "light.l1", Name: "Loft Ring", State: "off", AreaID: "loft", Area: "Loft"},
}

var plugs = []homeassistant.Entity{
	{ID: "switch.k", Name: "Kettle", State: "on", AreaID: "kitchen", Area: "Kitchen"},
	{ID: "switch.a", Name: "Attic Fan", State: "off", AreaID: "attic", Area: "Attic"},
}

type sent struct {
	action string
	ids    string
}

func testBoard(t *testing.T) (*Board, chan sent) {
	t.Helper()
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	calls := make(chan sent, 4)
	c := &cache{
		got: map[string]*result{"lights": {entities: lit, at: time.Now()}, "switches": {entities: plugs, at: time.Now()}},
		fetch: func(context.Context, homeassistant.Filter) ([]homeassistant.Entity, []homeassistant.Label, error) {
			return lit, nil, nil
		},
		now:  time.Now,
		done: func() {},
	}
	var mu sync.Mutex
	followed := map[string]*states.Value{}
	b := &Board{
		open:   map[string]bool{},
		values: map[string]*states.Value{},
		from:   c,
		follow: func(entity, attribute string) *states.Value {
			mu.Lock()
			defer mu.Unlock()
			v := &states.Value{Entity: entity, Attribute: attribute}
			followed[entity+"#"+attribute] = v
			return v
		},
		call: func(_ context.Context, action string, data map[string]string) error {
			calls <- sent{action, data["entity_id"]}
			return nil
		},
		redraw: func() {},
	}
	return b, calls
}

func lights() Selection { return Selections[0] }

func expect(t *testing.T, calls chan sent, want sent) {
	t.Helper()
	select {
	case got := <-calls:
		if got != want {
			t.Errorf("sent %+v, want %+v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("nothing was sent, want %+v", want)
	}
}

func TestTilesFollowThePick(t *testing.T) {
	b, _ := testBoard(t)
	if err := config.Set().Home().Pick("lights", config.HomePick{Areas: []string{"kitchen"}}); err != nil {
		t.Fatal(err)
	}
	areas, loading := b.Tiles(lights())
	if loading || len(areas) != 1 {
		t.Fatalf("areas %+v loading %v", areas, loading)
	}
	k := areas[0]
	if k.Name != "Kitchen" || k.On != 1 || len(k.Tiles) != 3 || k.Tiles[2].Available {
		t.Errorf("kitchen %+v", k)
	}
}

func TestAreaTapTurnsOffWhenAnyIsOn(t *testing.T) {
	b, calls := testBoard(t)
	if err := config.Set().Home().Pick("lights", config.HomePick{All: true}); err != nil {
		t.Fatal(err)
	}
	if err := config.Set().Home().Control("lights", true); err != nil {
		t.Fatal(err)
	}
	areas, _ := b.Tiles(lights())

	b.ToggleArea(areas[0])
	expect(t, calls, sent{"light.turn_off", "light.k1,light.k2"})

	b.ToggleArea(areas[1])
	expect(t, calls, sent{"light.turn_on", "light.l1"})

	b.ToggleEntity(areas[0].Tiles[1])
	expect(t, calls, sent{"light.turn_on", "light.k2"})
}

func TestViewOnlyWithoutControl(t *testing.T) {
	b, calls := testBoard(t)
	if err := config.Set().Home().Pick("lights", config.HomePick{All: true}); err != nil {
		t.Fatal(err)
	}
	areas, _ := b.Tiles(lights())
	b.ToggleArea(areas[0])
	b.ToggleEntity(areas[0].Tiles[0])
	select {
	case got := <-calls:
		t.Errorf("sent %+v with control off", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestTabsAndShowing(t *testing.T) {
	b, _ := testBoard(t)
	if tabs := b.Tabs(); len(tabs) != 0 {
		t.Errorf("tabs %v with nothing picked", tabs)
	}
	if err := config.Set().Home().Pick("lights", config.HomePick{Entities: []string{"light.k1"}}); err != nil {
		t.Fatal(err)
	}
	if tabs := b.Tabs(); len(tabs) != 0 {
		t.Errorf("tabs %v with control off", tabs)
	}
	if err := config.Set().Home().Control("lights", true); err != nil {
		t.Fatal(err)
	}
	if tabs := b.Tabs(); len(tabs) != 1 || tabs[0].Key != "lights" {
		t.Errorf("tabs %v", tabs)
	}
	b.Show("lights")
	if s, ok := b.Showing(); !ok || s.Key != "lights" {
		t.Errorf("showing %v %v", s, ok)
	}
	b.Show("lights")
	if _, ok := b.Showing(); ok {
		t.Error("tapping the open tab again should go back to the clock")
	}
	b.Show("lights")
	b.Clock()
	if _, ok := b.Showing(); ok {
		t.Error("Clock left a tab open")
	}
}

func TestCombineMergesSwitchesIntoLights(t *testing.T) {
	b, calls := testBoard(t)
	for _, key := range []string{"lights", "switches"} {
		if err := config.Set().Home().Pick(key, config.HomePick{All: true}); err != nil {
			t.Fatal(err)
		}
		if err := config.Set().Home().Control(key, true); err != nil {
			t.Fatal(err)
		}
	}
	if tabs := b.Tabs(); len(tabs) != 2 {
		t.Fatalf("tabs %v before combining", tabs)
	}
	if err := config.Set().Home().Combine(true); err != nil {
		t.Fatal(err)
	}
	if tabs := b.Tabs(); len(tabs) != 1 || tabs[0].Key != "lights" {
		t.Fatalf("tabs %v after combining", tabs)
	}

	areas, _ := b.Tiles(lights())
	var names []string
	for _, a := range areas {
		names = append(names, a.Name)
	}
	if len(areas) != 3 || names[0] != "Attic" || names[1] != "Kitchen" || names[2] != "Loft" {
		t.Fatalf("areas %v", names)
	}
	kitchen := areas[1]
	if len(kitchen.Tiles) != 4 || kitchen.On != 2 {
		t.Errorf("kitchen %+v", kitchen)
	}

	b.ToggleArea(kitchen)
	got := map[sent]bool{}
	for range 2 {
		select {
		case c := <-calls:
			got[c] = true
		case <-time.After(2 * time.Second):
			t.Fatal("an action never went out")
		}
	}
	for _, want := range []sent{{"light.turn_off", "light.k1,light.k2"}, {"switch.turn_off", "switch.k"}} {
		if !got[want] {
			t.Errorf("missing %+v in %v", want, got)
		}
	}

	if err := config.Set().Home().Control("switches", false); err != nil {
		t.Fatal(err)
	}
	if Combined() {
		t.Error("combined with switches no longer controlled")
	}
}
