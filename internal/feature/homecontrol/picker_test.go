package homecontrol

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
)

var house = []homeassistant.Entity{
	{ID: "light.k1", Name: "Kitchen Center", AreaID: "kitchen", Area: "Kitchen", Labels: []string{"panel"}},
	{ID: "light.k2", Name: "Kitchen Ring", AreaID: "kitchen", Area: "Kitchen"},
	{ID: "light.l1", Name: "Loft Ring", AreaID: "loft", Area: "Loft"},
	{ID: "light.x", Name: "Porch", State: "unavailable"},
}

var tags = []homeassistant.Label{{ID: "panel", Name: "Panel"}, {ID: "spare", Name: "Spare"}}

type fakeCache struct {
	c     *cache
	calls int
	mu    sync.Mutex
	fail  error
	clock time.Time
	ready chan struct{}
}

func newFake() *fakeCache {
	f := &fakeCache{clock: time.Unix(1000, 0), ready: make(chan struct{}, 8)}
	f.c = &cache{
		got: map[string]*result{},
		fetch: func(context.Context, homeassistant.Filter) ([]homeassistant.Entity, []homeassistant.Label, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.calls++
			if f.fail != nil {
				return nil, nil, f.fail
			}
			return house, tags, nil
		},
		now:  func() time.Time { f.mu.Lock(); defer f.mu.Unlock(); return f.clock },
		done: func() { f.ready <- struct{}{} },
	}
	return f
}

func (f *fakeCache) wait(t *testing.T) {
	t.Helper()
	select {
	case <-f.ready:
	case <-time.After(2 * time.Second):
		t.Fatal("the fetch never finished")
	}
}

func picker(f *fakeCache, pick *config.HomePick) *Picker {
	p := &Picker{Sel: Selections[0], tab: TabManual, open: map[string]bool{}, from: f.c}
	p.save = func(next config.HomePick) { *pick = next }
	return p
}

func loaded(t *testing.T, p *Picker, f *fakeCache) Snapshot {
	t.Helper()
	if s := p.Snapshot(); !s.Loading {
		return s
	}
	f.wait(t)
	return p.Snapshot()
}

func TestIncludes(t *testing.T) {
	cases := []struct {
		pick config.HomePick
		want []string
	}{
		{config.HomePick{}, nil},
		{config.HomePick{All: true}, []string{"light.k1", "light.k2", "light.l1", "light.x"}},
		{config.HomePick{Labels: []string{"panel"}}, []string{"light.k1"}},
		{config.HomePick{Areas: []string{"kitchen"}}, []string{"light.k1", "light.k2"}},
		{config.HomePick{Areas: []string{""}}, nil},
		{config.HomePick{Entities: []string{"light.l1"}, Labels: []string{"panel"}}, []string{"light.k1", "light.l1"}},
	}
	for _, c := range cases {
		var got []string
		for _, e := range house {
			if Includes(c.pick, e) {
				got = append(got, e.ID)
			}
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%+v includes %v, want %v", c.pick, got, c.want)
		}
	}
}

func TestAreasGroupAndCount(t *testing.T) {
	f := newFake()
	pick := config.HomePick{}
	p := picker(f, &pick)
	_ = loaded(t, p, f)

	rows := areaRows(house, config.HomePick{Areas: []string{"kitchen"}, Entities: []string{"light.l1"}}, "", map[string]bool{"loft": true})
	if len(rows) != 3 {
		t.Fatalf("%d areas, want kitchen, loft and no area", len(rows))
	}
	k, l, none := rows[0], rows[1], rows[2]
	if !k.On || k.Included != 2 || k.Open {
		t.Errorf("kitchen %+v", k)
	}
	if l.On || l.Included != 1 || !l.Open || !l.Entities[0].On {
		t.Errorf("loft %+v", l)
	}
	if none.ID != "" || none.On || none.Included != 0 {
		t.Errorf("no area %+v", none)
	}
}

func TestSearchNarrowsAndOpens(t *testing.T) {
	rows := areaRows(house, config.HomePick{}, "ring", map[string]bool{})
	var ids []string
	for _, a := range rows {
		if !a.Open {
			t.Errorf("%s is closed while searching", a.Name)
		}
		for _, e := range a.Entities {
			ids = append(ids, e.ID)
		}
	}
	if !slices.Equal(ids, []string{"light.k2", "light.l1"}) {
		t.Errorf("ring found %v", ids)
	}

	rows = areaRows(house, config.HomePick{}, "kitchen", map[string]bool{})
	if len(rows) != 1 || len(rows[0].Entities) != 2 {
		t.Errorf("an area name should bring the whole area: %+v", rows)
	}

	labels := labelRows(tags, house, config.HomePick{Labels: []string{"spare"}}, "")
	if len(labels) != 2 || labels[0].Count != 1 || labels[0].On || labels[1].Count != 0 || !labels[1].On {
		t.Errorf("labels %+v", labels)
	}
}

func TestCacheHoldsThenExpires(t *testing.T) {
	f := newFake()
	pick := config.HomePick{}
	p := picker(f, &pick)

	loaded(t, p, f)
	p.Snapshot()
	if f.calls != 1 {
		t.Fatalf("%d fetches inside the cache window, want 1", f.calls)
	}

	f.mu.Lock()
	f.clock = f.clock.Add(cacheFor + time.Second)
	f.mu.Unlock()
	loaded(t, p, f)
	if f.calls != 2 {
		t.Errorf("%d fetches after the window, want 2", f.calls)
	}
}

func TestErrorsAreNotCachedPastRetry(t *testing.T) {
	f := newFake()
	f.fail = errors.New("down")
	pick := config.HomePick{}
	p := picker(f, &pick)

	if s := loaded(t, p, f); s.Err == nil {
		t.Fatal("the failure did not show")
	}
	if s := p.Snapshot(); s.Err == nil || f.calls != 1 {
		t.Errorf("an error should stay until retried: err %v, %d fetches", s.Err, f.calls)
	}

	f.mu.Lock()
	f.fail = nil
	f.mu.Unlock()
	f.c.forget(p.Sel.Key)
	f.wait(t)
	if s := loaded(t, p, f); s.Err != nil || len(s.Areas) == 0 {
		t.Errorf("after retry: err %v, %d areas", s.Err, len(s.Areas))
	}
}

func TestClearForgetsEverySelection(t *testing.T) {
	f := newFake()
	pick := config.HomePick{}
	p := picker(f, &pick)
	loaded(t, p, f)

	f.c.clear()
	f.wait(t)
	loaded(t, p, f)
	if f.calls != 2 {
		t.Errorf("%d fetches, want a fresh one after clear", f.calls)
	}
}

func TestFlip(t *testing.T) {
	ids := []string{"a", "b"}
	if got := flip(ids, "a"); !slices.Equal(got, []string{"b"}) {
		t.Errorf("removing a gave %v", got)
	}
	if got := flip(ids, "c"); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("adding c gave %v", got)
	}
	if !slices.Equal(ids, []string{"a", "b"}) {
		t.Errorf("flip changed its input to %v", ids)
	}
}
