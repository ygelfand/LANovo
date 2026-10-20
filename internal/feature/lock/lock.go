package lock

import (
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/display/lockview"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	setting "github.com/ygelfand/libcountertop/pkg/settings"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

func init() { component.Register(sharedcomponent.Device, Get) }

type Lock struct {
	knobs *setting.Controls[schema.Lock]
	gate  *lockview.Gate
}

var get = sync.OnceValue(func() *Lock {
	l := &Lock{knobs: lockview.Controls(0, read, save)}
	l.gate = lockview.New(lockview.Options{
		Shell:  shell.Get(),
		Read:   read,
		Save:   save,
		SetPIN: func(pin string) error { return l.knobs.Change("pin", pin) },
	})
	component.Settings.Add(l.knobs)
	return l
})

func Get() *Lock { return get() }

func (l *Lock) Name() string { return "lock" }

func (l *Lock) Entities() []esphome.Entity { return l.knobs.Entities() }

func (l *Lock) Restore(cfg config.Config) { l.knobs.PublishFrom(cfg.Lock) }

func (l *Lock) Gate() *lockview.Gate { return l.gate }

func read() schema.Lock { return config.Get().Lock }

func save(at schema.Lock) error { return config.Set().Lock().All(at) }
