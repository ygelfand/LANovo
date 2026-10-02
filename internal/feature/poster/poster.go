// Package poster puts a picture from Immich behind the dashboard.
package poster

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	"github.com/ygelfand/LANovo/internal/lib/hook"
	"github.com/ygelfand/LANovo/internal/lib/immich"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

const (
	check      = 30 * time.Second
	retryAfter = 5 * time.Minute
	awayFor    = 5 * time.Minute
	darkLux    = 3
	lightLux   = 8
	candidates = 8
)

func init() {
	component.Register(component.Device, Get, component.Order(25))
}

type Poster struct {
	on      *esphome.Switch
	every   *esphome.Select
	server  *esphome.Text
	key     *esphome.Text
	albums  *esphome.Text
	tags    *esphome.Text
	next    *esphome.Button
	showing *esphome.TextSensor

	Changed hook.Hook[int]

	kick chan struct{}

	mu      sync.Mutex
	src     image.Image
	version int
	shownAt time.Time
	failed  time.Time
	lastErr string
	made    *image.RGBA
	madeFor string

	fetch func(ctx context.Context, cfg config.Poster) (id string, data []byte, err error)
}

var (
	once   sync.Once
	shared *Poster
)

func Get() *Poster {
	once.Do(func() {
		shared = &Poster{kick: make(chan struct{}, 1)}
		shared.fetch = fetchFromImmich
		shared.build()
	})
	return shared
}

func (p *Poster) Name() string { return "poster" }

func (p *Poster) Entities() []esphome.Entity {
	return []esphome.Entity{p.on, p.every, p.server, p.key, p.albums, p.tags, p.next, p.showing}
}

func (p *Poster) Restore(cfg config.Config) {
	c := cfg.Poster
	p.on.Set(c.Enabled)
	p.every.Set(c.Every.Label())
	p.server.Set(c.Server)
	p.key.Set(c.Key)
	p.albums.Set(c.Albums)
	p.tags.Set(c.Tags)
	p.showing.Set(c.Last)
}

func (p *Poster) Run(ctx context.Context) error {
	t := time.NewTicker(check)
	defer t.Stop()
	away := false
	for {
		kicked := false
		select {
		case <-ctx.Done():
			return nil
		case <-p.kick:
			kicked = true
		case <-t.C:
		}
		woke := false
		if since := touch.Get().Since(); since > awayFor {
			away = true
		} else if away && since < check {
			away, woke = false, true
		}
		p.step(ctx, time.Now(), kicked, woke)
	}
}

func (p *Poster) step(ctx context.Context, now time.Time, kicked, woke bool) {
	cfg := config.Get().Poster
	if !cfg.Enabled || cfg.Server == "" {
		p.drop()
		return
	}
	p.mu.Lock()
	have, shownAt, failed := p.src != nil, p.shownAt, p.failed
	p.mu.Unlock()

	due := !have || kicked
	if have && !due {
		switch cfg.Every {
		case config.PosterWake:
			due = woke
		case config.PosterHourly, config.PosterDaily:
			due = now.Sub(shownAt) >= cfg.Every.Period()
		}
	}
	if !due || (!kicked && !failed.IsZero() && now.Sub(failed) < retryAfter) {
		return
	}

	id, data, err := p.fetch(ctx, cfg)
	var src image.Image
	if err == nil {
		src, err = decode(data)
	}
	if err != nil {
		p.mu.Lock()
		p.failed = now
		repeat := p.lastErr == err.Error()
		p.lastErr = err.Error()
		p.mu.Unlock()
		if !repeat {
			slog.Warn("fetching a poster failed", "err", err)
		}
		return
	}
	p.mu.Lock()
	p.src, p.shownAt, p.failed, p.lastErr = src, now, time.Time{}, ""
	p.version++
	p.made, p.madeFor = nil, ""
	v := p.version
	p.mu.Unlock()
	if err := config.Set().Poster().Last(id); err != nil {
		slog.Error("saving a setting failed", "setting", "poster last", "err", err)
	}
	p.showing.Set(id)
	slog.Info("poster changed", "asset", id)
	p.Changed.Emit(v)
}

func (p *Poster) drop() {
	p.mu.Lock()
	had := p.src != nil
	p.src, p.made, p.madeFor = nil, nil, ""
	p.version++
	v := p.version
	p.mu.Unlock()
	if had {
		p.Changed.Emit(v)
	}
}

var dark atomic.Bool

func Dark() bool {
	lux, ok := sensors.Get().Ambient()
	return settle(&dark, lux, ok)
}

func settle(state *atomic.Bool, lux float64, ok bool) bool {
	switch {
	case !ok:
		state.Store(false)
	case lux < darkLux:
		state.Store(true)
	case lux > lightLux:
		state.Store(false)
	}
	return state.Load()
}

// Backdrop is the picture for a w×h screen with the clock at box, and a key that changes whenever it does.
func (p *Poster) Backdrop(w, h int, box image.Rectangle, bg theme.Color) (*image.RGBA, string) {
	if Dark() {
		return nil, "dark"
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.src == nil {
		return nil, ""
	}
	key := fmt.Sprintf("v%d %dx%d %v %v", p.version, w, h, box, bg)
	if p.madeFor != key {
		p.made, p.madeFor = compose(p.src, w, h, box, bg), key
	}
	return p.made, key
}

func (p *Poster) Next() {
	select {
	case p.kick <- struct{}{}:
	default:
	}
}

func fetchFromImmich(ctx context.Context, cfg config.Poster) (string, []byte, error) {
	c := immich.New(cfg.Server, cfg.Key)
	albums, tags, err := c.Resolve(ctx, config.Names(cfg.Albums), config.Names(cfg.Tags))
	if err != nil {
		return "", nil, err
	}
	found, err := c.Random(ctx, albums, tags, candidates)
	if err != nil {
		return "", nil, err
	}
	id := pick(found, cfg.Last)
	if id == "" {
		return "", nil, errors.New("poster: no pictures match the albums and tags")
	}
	data, err := c.Preview(ctx, id)
	return id, data, err
}

func pick(found []immich.Asset, last string) string {
	first := ""
	for _, a := range found {
		if a.Type != "" && a.Type != "IMAGE" {
			continue
		}
		if first == "" {
			first = a.ID
		}
		if a.ID != last {
			return a.ID
		}
	}
	return first
}

func (p *Poster) build() {
	base := func(id, name, icon string) esphome.Base {
		return esphome.Base{ObjectID: id, Name: name, Icon: icon, Category: esphome.CategoryConfig, DeviceID: component.DeviceScreen}
	}
	p.on = &esphome.Switch{Base: base("poster", "Poster", "mdi:image-frame")}
	p.on.OnCommand = p.SetEnabled

	p.every = &esphome.Select{Base: base("poster_every", "Poster changes", "mdi:image-sync"), Options: config.Labels(config.PosterEveries())}
	p.every.OnCommand = func(label string) {
		if v, ok := config.ByLabel(config.PosterEveries(), label); ok {
			p.SetEvery(v)
		}
	}

	p.server = &esphome.Text{Base: base("poster_server", "Immich server", "mdi:server"), MaxLength: 255}
	p.server.OnCommand = p.SetServer
	p.key = &esphome.Text{Base: base("poster_key", "Immich API key", "mdi:key"), MaxLength: 255, Mode: api.TextMode_TEXT_MODE_PASSWORD}
	p.key.OnCommand = p.SetKey
	p.albums = &esphome.Text{Base: base("poster_albums", "Poster albums", "mdi:image-album"), MaxLength: 255}
	p.albums.OnCommand = p.SetAlbums
	p.tags = &esphome.Text{Base: base("poster_tags", "Poster tags", "mdi:tag-multiple"), MaxLength: 255}
	p.tags.OnCommand = p.SetTags

	p.next = &esphome.Button{Base: base("poster_next", "Next poster", "mdi:skip-next")}
	p.next.OnPress = p.Next

	p.showing = &esphome.TextSensor{Base: esphome.Base{ObjectID: "poster_showing", Name: "Poster showing", Icon: "mdi:image",
		Category: esphome.CategoryDiagnostic, DeviceID: component.DeviceScreen}}
}

func (p *Poster) saved(what string, err error) {
	if err != nil {
		slog.Error("saving a setting failed", "setting", what, "err", err)
		return
	}
	p.Next()
}

func (p *Poster) SetEnabled(on bool) {
	p.on.Set(on)
	p.saved("poster", config.Set().Poster().Enabled(on))
}

func (p *Poster) SetEvery(v config.PosterEvery) {
	p.every.Set(v.Label())
	if err := config.Set().Poster().Every(v); err != nil {
		slog.Error("saving a setting failed", "setting", "poster every", "err", err)
	}
}

func (p *Poster) SetServer(v string) {
	p.server.Set(v)
	p.saved("poster server", config.Set().Poster().Server(v))
}

func (p *Poster) SetKey(v string) {
	p.key.Set(v)
	p.saved("poster key", config.Set().Poster().Key(v))
}

func (p *Poster) SetAlbums(v string) {
	p.albums.Set(v)
	p.saved("poster albums", config.Set().Poster().Albums(v))
}

func (p *Poster) SetTags(v string) {
	p.tags.Set(v)
	p.saved("poster tags", config.Set().Poster().Tags(v))
}
