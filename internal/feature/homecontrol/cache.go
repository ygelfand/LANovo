package homecontrol

import (
	"context"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

const (
	fetchWait = 10 * time.Second
	cacheFor  = 10 * time.Minute
)

type result struct {
	entities []homeassistant.Entity
	labels   []homeassistant.Label
	err      error
	at       time.Time
	asking   bool
}

type cache struct {
	mu    sync.Mutex
	got   map[string]*result
	fetch func(context.Context, homeassistant.Filter) ([]homeassistant.Entity, []homeassistant.Label, error)
	now   func() time.Time
	done  func()
}

var shared = &cache{
	got:   map[string]*result{},
	fetch: fromHomeAssistant,
	now:   time.Now,
	done:  func() { shell.Get().Redraw() },
}

func init() {
	homeassistant.Get().Changed.Listen(func(a homeassistant.Access) {
		if a == homeassistant.Allowed {
			shared.retry()
		}
	})
}

func fromHomeAssistant(ctx context.Context, f homeassistant.Filter) ([]homeassistant.Entity, []homeassistant.Label, error) {
	ha := homeassistant.Get()
	entities, err := ha.Entities(ctx, f)
	if err != nil {
		return nil, nil, err
	}
	labels, err := ha.Labels(ctx)
	return entities, labels, err
}

func (c *cache) get(s Selection) result {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.got[s.Key]
	if r == nil || (!r.asking && r.err == nil && c.now().Sub(r.at) > cacheFor) {
		r = &result{asking: true}
		c.got[s.Key] = r
		go c.load(s, r)
	}
	return *r
}

func (c *cache) load(s Selection, r *result) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchWait)
	defer cancel()
	entities, labels, err := c.fetch(ctx, s.Filter)
	if err == nil && entities == nil {
		entities = []homeassistant.Entity{}
	}
	c.mu.Lock()
	if c.got[s.Key] == r {
		c.got[s.Key] = &result{entities: entities, labels: labels, err: err, at: c.now()}
	}
	c.mu.Unlock()
	c.done()
}

func (c *cache) forget(key string) {
	c.mu.Lock()
	delete(c.got, key)
	c.mu.Unlock()
	c.done()
}

func (c *cache) retry() {
	c.mu.Lock()
	for k, r := range c.got {
		if r.err != nil {
			delete(c.got, k)
		}
	}
	c.mu.Unlock()
	c.done()
}

func (c *cache) clear() {
	c.mu.Lock()
	c.got = map[string]*result{}
	c.mu.Unlock()
	c.done()
}

func Refresh() {
	shared.clear()
	homeassistant.Get().Probe()
}
