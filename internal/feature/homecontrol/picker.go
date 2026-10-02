package homecontrol

import (
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

type Tab string

const (
	TabLabels Tab = "labels"
	TabManual Tab = "manual"
)

type Picker struct {
	Sel Selection

	mu    sync.Mutex
	tab   Tab
	open  map[string]bool
	query string

	from *cache
	save func(config.HomePick)
}

func NewPicker(s Selection) *Picker {
	p := &Picker{Sel: s, tab: TabManual, open: map[string]bool{}, from: shared}
	p.save = func(pick config.HomePick) {
		if err := config.Set().Home().Pick(s.Key, pick); err != nil {
			slog.Error("the Home Assistant selection could not be saved", "selection", s.Key, "err", err)
		}
		shell.Get().Redraw()
	}
	return p
}

func (*Picker) Covers() bool { return true }

func (*Picker) Timeout() time.Duration { return shell.SettingsTimeout }

type LabelRow struct {
	ID    string
	Name  string
	Count int
	On    bool
}

type EntityRow struct {
	homeassistant.Entity
	On       bool
	Included bool
}

type AreaRow struct {
	ID       string
	Name     string
	Open     bool
	On       bool
	Included int
	Entities []EntityRow
}

type Snapshot struct {
	Loading bool
	Err     error
	Tab     Tab
	Query   string
	Pick    config.HomePick
	Labels  []LabelRow
	Areas   []AreaRow
}

func (p *Picker) Snapshot() Snapshot {
	got := p.from.get(p.Sel)
	p.mu.Lock()
	s := Snapshot{Loading: got.asking, Err: got.err, Tab: p.tab, Query: p.query, Pick: p.Sel.Pick()}
	query, open := strings.ToLower(strings.TrimSpace(p.query)), p.open
	p.mu.Unlock()

	if s.Loading || s.Err != nil {
		return s
	}
	s.Labels = labelRows(got.labels, got.entities, s.Pick, query)
	s.Areas = areaRows(got.entities, s.Pick, query, open)
	return s
}

func matches(query string, names ...string) bool {
	if query == "" {
		return true
	}
	return slices.ContainsFunc(names, func(n string) bool { return strings.Contains(strings.ToLower(n), query) })
}

func labelRows(labels []homeassistant.Label, entities []homeassistant.Entity, pick config.HomePick, query string) []LabelRow {
	var rows []LabelRow
	for _, l := range labels {
		if !matches(query, l.Name) {
			continue
		}
		n := 0
		for _, e := range entities {
			if slices.Contains(e.Labels, l.ID) {
				n++
			}
		}
		rows = append(rows, LabelRow{ID: l.ID, Name: l.Name, Count: n, On: slices.Contains(pick.Labels, l.ID)})
	}
	return rows
}

func areaRows(entities []homeassistant.Entity, pick config.HomePick, query string, open map[string]bool) []AreaRow {
	var rows []AreaRow
	at := map[string]int{}
	for _, e := range entities {
		areaHit := matches(query, e.Area)
		if !areaHit && !matches(query, e.Name, e.ID) {
			continue
		}
		i, ok := at[e.AreaID]
		if !ok {
			i = len(rows)
			at[e.AreaID] = i
			rows = append(rows, AreaRow{
				ID:   e.AreaID,
				Name: e.Area,
				Open: open[e.AreaID] || query != "",
				On:   e.AreaID != "" && slices.Contains(pick.Areas, e.AreaID),
			})
		}
		row := EntityRow{Entity: e, On: slices.Contains(pick.Entities, e.ID), Included: Includes(pick, e)}
		if row.Included {
			rows[i].Included++
		}
		rows[i].Entities = append(rows[i].Entities, row)
	}
	return rows
}

func (p *Picker) Retry() { p.from.forget(p.Sel.Key) }

func (p *Picker) SetTab(t Tab) {
	p.mu.Lock()
	p.tab = t
	p.mu.Unlock()
	shell.Get().Redraw()
}

func (p *Picker) SetQuery(q string) {
	p.mu.Lock()
	p.query = q
	p.mu.Unlock()
	shell.Get().Redraw()
}

func (p *Picker) Expand(area string) {
	p.mu.Lock()
	open := make(map[string]bool, len(p.open)+1)
	for k, v := range p.open {
		open[k] = v
	}
	open[area] = !open[area]
	p.open = open
	p.mu.Unlock()
	shell.Get().Redraw()
}

func (p *Picker) ToggleAll() {
	pick := p.Sel.Pick()
	pick.All = !pick.All
	p.save(pick)
}

func (p *Picker) ToggleLabel(id string) {
	pick := p.Sel.Pick()
	pick.Labels = flip(pick.Labels, id)
	p.save(pick)
}

func (p *Picker) ToggleArea(id string) {
	if id == "" {
		return
	}
	pick := p.Sel.Pick()
	pick.Areas = flip(pick.Areas, id)
	p.save(pick)
}

func (p *Picker) ToggleEntity(id string) {
	pick := p.Sel.Pick()
	pick.Entities = flip(pick.Entities, id)
	p.save(pick)
}

func flip(ids []string, id string) []string {
	if i := slices.Index(ids, id); i >= 0 {
		return slices.Delete(slices.Clone(ids), i, i+1)
	}
	return append(slices.Clone(ids), id)
}
