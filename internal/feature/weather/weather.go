package weather

import (
	"context"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/states"
)

const (
	attrTemperature = "temperature"
	attrUnit        = "temperature_unit"
	attrHumidity    = "humidity"
	attrFeels       = "apparent_temperature"
	attrWind        = "wind_speed"
	attrWindUnit    = "wind_speed_unit"
	attrUV          = "uv_index"
)

var attributes = []string{"", attrTemperature, attrUnit, attrHumidity, attrFeels, attrWind, attrWindUnit, attrUV}

type Reading struct {
	Condition   string
	Temperature *float64
	Unit        string
	Humidity    *float64
	Feels       *float64
	Wind        *float64
	WindUnit    string
	UV          *float64
	Phase       float64
}

type Weather struct {
	mu     sync.Mutex
	entity string
	values map[string]*states.Value

	ha entities

	offered  []homeassistant.Entity
	failed   error
	fetching bool
	fetched  bool
}

const fetchWait = 10 * time.Second

func (w *Weather) Offered() ([]homeassistant.Entity, error, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.offered, w.failed, w.fetched
}

func (w *Weather) Fetch() {
	w.mu.Lock()
	if w.fetching {
		w.mu.Unlock()
		return
	}
	w.fetching = true
	w.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), fetchWait)
		defer cancel()
		list, err := homeassistant.Get().Entities(ctx, homeassistant.Filter{Domains: []string{"weather"}})
		w.mu.Lock()
		w.offered, w.failed, w.fetching, w.fetched = list, err, false, true
		w.mu.Unlock()
		if err != nil {
			slog.Info("weather entities", "err", err)
		}
		if err == nil && len(list) > 0 && strings.TrimSpace(config.Get().Weather.Entity) == "" {
			SetEntity(list[0].ID)
		}
		shell.Get().Redraw()
	}()
}

func (w *Weather) Title(entity string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, e := range w.offered {
		if e.ID == entity && e.Name != "" {
			return e.Name
		}
	}
	return entity
}

var (
	once   sync.Once
	shared *Weather
)

func Get() *Weather {
	once.Do(func() {
		shared = &Weather{}
		shared.build()
	})
	return shared
}

func (w *Weather) Now() (Reading, bool) {
	cfg := config.Get()
	entity := strings.TrimSpace(cfg.Weather.Entity)
	if !cfg.Home.Enabled {
		return Reading{}, false
	}
	if entity == "" {
		if _, _, fetched := w.Offered(); !fetched && homeassistant.Get().Access() == homeassistant.Allowed {
			w.Fetch()
		}
		return Reading{}, false
	}
	values := w.follow(entity)
	condition, ok := values[""].Get()
	if !ok {
		return Reading{}, false
	}
	text := func(a string) string {
		s, _ := values[a].Get()
		return strings.TrimSpace(s)
	}
	number := func(a string) *float64 {
		f, ok := values[a].Float()
		if !ok || math.IsNaN(f) {
			return nil
		}
		return &f
	}
	return Reading{
		Condition:   condition,
		Temperature: number(attrTemperature),
		Unit:        text(attrUnit),
		Humidity:    number(attrHumidity),
		Feels:       number(attrFeels),
		Wind:        number(attrWind),
		WindUnit:    text(attrWindUnit),
		UV:          number(attrUV),
	}, true
}

func (w *Weather) follow(entity string) map[string]*states.Value {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.entity == entity {
		return w.values
	}
	values := make(map[string]*states.Value, len(attributes))
	for _, a := range attributes {
		v := states.Get().Follow(entity, a)
		v.Changed.Listen(func(string) { shell.Get().Redraw() })
		values[a] = v
	}
	w.entity, w.values = entity, values
	slog.Info("weather following", "entity", entity)
	return values
}

func save(what string, err error) {
	if err != nil {
		slog.Error("saving a weather setting failed", "setting", what, "err", err)
	}
	Get().publish(config.Get().Weather)
	shell.Get().Redraw()
}

func SetEntity(v string) { save("entity", config.Set().Weather().Entity(strings.TrimSpace(v))) }

func SetLook(v config.WeatherLook) { save("look", config.Set().Weather().Look(v)) }

func SetDashboard(on bool) { save("dashboard", config.Set().Weather().Dashboard(on)) }

func SetIdle(on bool) { save("idle", config.Set().Weather().Idle(on)) }

func SetAnimate(on bool) { save("animate", config.Set().Weather().Animate(on)) }

func SetThemed(on bool) { save("themed", config.Set().Weather().Themed(on)) }

func SetPosition(v config.Position) { save("position", config.Set().Weather().Position(v)) }

func SetAlign(v config.Align) { save("align", config.Set().Weather().Align(v)) }

func SetSize(v config.Size) { save("size", config.Set().Weather().Size(v)) }
