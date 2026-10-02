package homecontrol

import (
	"cmp"
	"context"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/dashboard"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/states"
)

const (
	callWait = 10 * time.Second
	TabKind  = "home"
)

func init() {
	dashboard.AddTabs(func() []dashboard.Tab {
		var out []dashboard.Tab
		for _, s := range Dash().Tabs() {
			out = append(out, dashboard.Tab{Kind: TabKind, Key: s.Key, Name: s.Name()})
		}
		return out
	})
}

type Tile struct {
	ID        string
	Name      string
	On        bool
	Available bool
	Level     int
}

type AreaTiles struct {
	ID    string
	Name  string
	Open  bool
	On    int
	Tiles []Tile
}

type Board struct {
	mu     sync.Mutex
	open   map[string]bool
	values map[string]*states.Value

	from   *cache
	follow func(entity, attribute string) *states.Value
	call   func(ctx context.Context, action string, data map[string]string) error
	redraw func()
}

var (
	board     *Board
	boardOnce sync.Once
)

func Dash() *Board {
	boardOnce.Do(func() {
		board = &Board{
			open:   map[string]bool{},
			values: map[string]*states.Value{},
			from:   shared,
			follow: states.Get().Follow,
			call:   homeassistant.Get().Call,
			redraw: func() { shell.Get().Redraw() },
		}
	})
	return board
}

func (b *Board) Tabs() []Selection {
	var out []Selection
	for _, s := range Selections {
		if s.Controlled() && !s.Pick().Empty() && !(s.Key == "switches" && Combined()) {
			out = append(out, s)
		}
	}
	return out
}

func (b *Board) sources(s Selection) []Selection {
	out := []Selection{s}
	if s.Key == "lights" && Combined() {
		for _, other := range Selections {
			if other.Key == "switches" && !other.Pick().Empty() {
				out = append(out, other)
			}
		}
	}
	return out
}

func (b *Board) Find(key string) (Selection, bool) {
	for _, s := range b.Tabs() {
		if s.Key == key {
			return s, true
		}
	}
	return Selection{}, false
}

func (b *Board) Expand(s Selection, area string) {
	b.mu.Lock()
	open := make(map[string]bool, len(b.open)+1)
	for k, v := range b.open {
		open[k] = v
	}
	open[s.Key+"/"+area] = !open[s.Key+"/"+area]
	b.open = open
	b.mu.Unlock()
	b.redraw()
}

func (b *Board) Tiles(s Selection) ([]AreaTiles, bool) {
	b.mu.Lock()
	open := b.open
	b.mu.Unlock()

	var areas []AreaTiles
	at := map[string]int{}
	for _, src := range b.sources(s) {
		got := b.from.get(src)
		if got.asking {
			return nil, true
		}
		pick := src.Pick()
		for _, e := range got.entities {
			if !Includes(pick, e) {
				continue
			}
			i, ok := at[e.AreaID]
			if !ok {
				i = len(areas)
				at[e.AreaID] = i
				areas = append(areas, AreaTiles{ID: e.AreaID, Name: e.Area, Open: open[s.Key+"/"+e.AreaID]})
			}
			t := b.tile(e)
			if t.On {
				areas[i].On++
			}
			areas[i].Tiles = append(areas[i].Tiles, t)
		}
	}
	slices.SortStableFunc(areas, func(x, y AreaTiles) int {
		if (x.ID == "") != (y.ID == "") {
			if x.ID == "" {
				return 1
			}
			return -1
		}
		return cmp.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})
	for i := range areas {
		slices.SortStableFunc(areas[i].Tiles, func(x, y Tile) int {
			return cmp.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
		})
	}
	return areas, false
}

func (b *Board) tile(e homeassistant.Entity) Tile {
	state := e.State
	if v, ok := b.value(e.ID, "").Get(); ok {
		state = v
	}
	t := Tile{ID: e.ID, Name: e.Name, On: state == "on", Available: state != "unavailable", Level: -1}
	if t.Name == "" {
		t.Name = e.ID
	}
	if e.Domain() == "light" && t.On {
		if raw, ok := b.value(e.ID, "brightness").Get(); ok {
			if n, err := strconv.ParseFloat(raw, 64); err == nil {
				t.Level = int(n*100/255 + 0.5)
			}
		}
	}
	return t
}

func (b *Board) value(entity, attribute string) *states.Value {
	key := entity + "#" + attribute
	b.mu.Lock()
	v := b.values[key]
	b.mu.Unlock()
	if v != nil {
		return v
	}
	v = b.follow(entity, attribute)
	b.mu.Lock()
	if have := b.values[key]; have != nil {
		b.mu.Unlock()
		return have
	}
	values := make(map[string]*states.Value, len(b.values)+1)
	for k, have := range b.values {
		values[k] = have
	}
	values[key] = v
	b.values = values
	b.mu.Unlock()
	v.Changed.Listen(func(string) { b.redraw() })
	return v
}

func (t Tile) Domain() string { return homeassistant.Entity{ID: t.ID}.Domain() }

func controls(domain string) bool {
	for _, s := range Selections {
		if slices.Contains(s.Filter.Domains, domain) {
			return s.Controlled()
		}
	}
	return false
}

func (b *Board) ToggleArea(area AreaTiles) {
	action := "turn_on"
	if area.On > 0 {
		action = "turn_off"
	}
	var ids []string
	for _, t := range area.Tiles {
		if t.Available && controls(t.Domain()) {
			ids = append(ids, t.ID)
		}
	}
	b.send(action, ids)
}

func (b *Board) ToggleEntity(t Tile) {
	if !t.Available || !controls(t.Domain()) {
		return
	}
	action := "turn_on"
	if t.On {
		action = "turn_off"
	}
	b.send(action, []string{t.ID})
}

func (b *Board) send(action string, ids []string) {
	byDomain := map[string][]string{}
	var order []string
	for _, id := range ids {
		d := homeassistant.Entity{ID: id}.Domain()
		if _, ok := byDomain[d]; !ok {
			order = append(order, d)
		}
		byDomain[d] = append(byDomain[d], id)
	}
	for _, d := range order {
		ids := byDomain[d]
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), callWait)
			defer cancel()
			if err := b.call(ctx, d+"."+action, map[string]string{"entity_id": strings.Join(ids, ",")}); err != nil {
				slog.Warn("Home Assistant did not take the action", "action", d+"."+action, "entities", len(ids), "err", err)
			}
		}()
	}
}
