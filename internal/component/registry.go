package component

import (
	"cmp"
	"context"
	"slices"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/service"
)

// Registry is everything registered, in the order it happens.
//
// Order is declared, never inferred. Components register from init, and Go decides that order from
// the import graph — which is not something the boot screen coming up before the dashboard should
// depend on. So entries sort by phase, then by a declared Order, then by name, and the result is the
// same whatever the linker did.
type Registry struct {
	mu      sync.Mutex
	entries []*entry
}

type entry struct {
	make  func() Component
	once  sync.Once
	c     Component
	phase Phase
	order int
	opts  []service.Option
}

// resolve builds the component, once. Every walk goes through sorted, which resolves before anything
// reads c.
func (e *entry) resolve() { e.once.Do(func() { e.c = e.make() }) }

// Option adjusts one registration.
type Option func(*entry)

// Order places a component within its phase. Lower comes first; equal orders fall back to the name.
func Order(n int) Option { return func(e *entry) { e.order = n } }

// Supervise passes the policy through to the supervisor, for a component with a loop.
func Supervise(opts ...service.Option) Option {
	return func(e *entry) { e.opts = append(e.opts, opts...) }
}

// New is an empty registry. There is a process-wide one for components to register into; this is
// for tests, which want their own.
func New() *Registry { return &Registry{} }

var shared = New()

// Register adds to the process-wide registry, from a component package's init.
//
// What is registered is the constructor, not the component: init runs on every invocation of the
// binary, and building a component opens the hardware it drives. Only the agent should do that.
func Register[T Component](p Phase, make func() T, opts ...Option) {
	shared.Add(p, func() Component { return make() }, opts...)
}

// Default is the process-wide registry.
func Default() *Registry { return shared }

// Add registers a component, built on first use and once.
func (r *Registry) Add(p Phase, make func() Component, opts ...Option) {
	e := &entry{make: make, phase: p}
	for _, o := range opts {
		o(e)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
}

// All is every component, in the order they come up.
func (r *Registry) All() []Component {
	sorted := r.sorted()
	out := make([]Component, 0, len(sorted))
	for _, e := range sorted {
		out = append(out, e.c)
	}
	return out
}

// Entities is everything the components show Home Assistant.
func (r *Registry) Entities() []esphome.Entity {
	var out []esphome.Entity
	for _, e := range r.sorted() {
		if ents, ok := e.c.(Entities); ok {
			out = append(out, ents.Entities()...)
		}
	}
	return out
}

// Actions is everything the components let Home Assistant call.
func (r *Registry) Actions() []*esphome.Action {
	var out []*esphome.Action
	for _, e := range r.sorted() {
		if acts, ok := e.c.(Actions); ok {
			out = append(out, acts.Actions()...)
		}
	}
	return out
}

// Handlers is the components that answer protocol messages themselves.
func (r *Registry) Handlers() []esphome.Handler {
	var out []esphome.Handler
	for _, e := range r.sorted() {
		if h, ok := e.c.(Handler); ok {
			out = append(out, h)
		}
	}
	return out
}

// Progress is what every component that has something to say about coming up is doing, in the
// order they start.
func (r *Registry) Progress() []Progress {
	var out []Progress
	for _, e := range r.sorted() {
		s, ok := e.c.(Startup)
		if !ok {
			continue
		}

		p := s.Startup()
		p.Name = e.c.Name()
		out = append(out, p)
	}
	return out
}

// Ready reports whether everything the boot screen waits for is up.
func (r *Registry) Ready() bool {
	for _, p := range r.Progress() {
		if !p.settled() && !p.Background {
			return false
		}
	}
	return true
}

// Restore puts every component back the way the device was left. Order matters and is the
// registration order: the hardware has to be where it was before anything shows what it is doing.
func (r *Registry) Restore(c config.Config) {
	for _, e := range r.sorted() {
		if v, ok := e.c.(Restorer); ok {
			v.Restore(c)
		}
	}
}

// Group is the components as a supervisor, for a caller with nothing to wire either side.
func (r *Registry) Group() *service.Group {
	g := service.New()
	r.AddTo(g)
	return g
}

// AddTo hands the components to a supervisor, in order. It adds rather than owning the group,
// because the group starts in add order and there is still hand-wired work either side.
//
// A component with no loop is added too, as long as it has something to start or close: plenty of
// what a device does at boot happens once and still belongs in the ordered list.
func (r *Registry) AddTo(g *service.Group) {
	for _, e := range r.sorted() {
		if svc, ok := e.c.(service.Service); ok {
			g.Add(svc, e.opts...)
			continue
		}

		_, starts := e.c.(Starter)
		_, closes := e.c.(Closer)
		if !starts && !closes {
			continue
		}
		g.Add(oneshot{e.c}, append([]service.Option{service.Once()}, e.opts...)...)
	}
}

// oneshot gives a component with no loop the Run the supervisor needs. Returning nil immediately is
// how service.Once reads a service that finished rather than one that died.
//
// Start and Close are forwarded by hand: embedding a Component promotes only the methods of that
// interface, so the wrapper would otherwise hide the very halves it exists to run.
type oneshot struct{ Component }

func (oneshot) Run(context.Context) error { return nil }

func (o oneshot) Start(ctx context.Context) error {
	if s, ok := o.Component.(Starter); ok {
		return s.Start(ctx)
	}
	return nil
}

func (o oneshot) Close() error {
	if c, ok := o.Component.(Closer); ok {
		return c.Close()
	}
	return nil
}

// sorted is the entries in the order everything walks them.
func (r *Registry) sorted() []*entry {
	r.mu.Lock()
	out := slices.Clone(r.entries)
	r.mu.Unlock()

	// Outside the lock: a constructor is the component's own code and has no business being run with
	// the registry held.
	for _, e := range out {
		e.resolve()
	}

	slices.SortStableFunc(out, func(a, b *entry) int {
		if v := cmp.Compare(a.phase, b.phase); v != 0 {
			return v
		}
		if v := cmp.Compare(a.order, b.order); v != 0 {
			return v
		}
		return cmp.Compare(a.c.Name(), b.c.Name())
	})
	return out
}
