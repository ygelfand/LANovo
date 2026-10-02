package service

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/safe"
)

// Group runs a set of services and keeps them running.
//
// Services are started in the order they were added and stopped in reverse, which is the only
// ordering there is: there is no dependency graph, because the order is short, fixed, and easier to
// read as a list than to derive. Anything that needs to wait for another service waits on a signal
// of its own rather than on being constructed later.
type Group struct {
	mu       sync.Mutex
	entries  []*entry
	started  bool
	stopping bool
}

type entry struct {
	svc    Service
	policy policy

	mu     sync.Mutex
	state  State
	err    error
	since  time.Time
	starts int
}

func New() *Group { return &Group{} }

// Add registers a service. Adding after Run has begun is refused loudly.
func (g *Group) Add(svc Service, opts ...Option) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.started {
		// Adding after Run would silently never start, which is worse than saying so.
		slog.Error("service added after the group started", "service", svc.Name())
		return
	}

	p := defaults()
	for _, o := range opts {
		o(&p)
	}
	g.entries = append(g.entries, &entry{svc: svc, policy: p, state: StateWaiting, since: time.Now()})
}

// Status is what every service is doing.
func (g *Group) Status() []Status {
	g.mu.Lock()
	entries := make([]*entry, len(g.entries))
	copy(entries, g.entries)
	g.mu.Unlock()

	out := make([]Status, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.status())
	}
	return out
}

// Run starts everything and blocks until ctx is canceled and every service has stopped.
//
// A required service that cannot be acquired is fatal: Run returns its error without starting what
// follows, because a device missing something it cannot work without is better off restarting than
// limping. Anything else that fails is recorded and the rest carry on.
func (g *Group) Run(ctx context.Context) error {
	g.mu.Lock()
	g.started = true
	entries := make([]*entry, len(g.entries))
	copy(entries, g.entries)
	g.mu.Unlock()

	var (
		wg      sync.WaitGroup
		started []*entry
	)

	for _, e := range entries {
		// Acquire before running, in order, so a service that has to take a device off Android does
		// it while nothing else is competing for it.
		began := time.Now()
		acquired := false

		if err := e.start(ctx); err != nil {
			e.set(StateFailed, err)
			if e.policy.required {
				slog.Error("required service unavailable, giving up", "service", e.svc.Name(), "err", err)
				g.stop(started, &wg)
				return fmt.Errorf("service: %s: %w", e.svc.Name(), err)
			}
			if !e.policy.restart {
				slog.Error("service unavailable, continuing without it",
					"service", e.svc.Name(), "err", err)
				continue
			}

			// Not there yet is not the same as not there. A device node ueventd has still to make
			// is the ordinary case this early, and giving up here loses the service for the life
			// of the process however long its restart policy says to keep trying.
			slog.Error("service unavailable, still asking for it",
				"service", e.svc.Name(), "err", err)
		} else {
			slog.Info("service started", "service", e.svc.Name(),
				"in", time.Since(began).Round(time.Millisecond))
			acquired = true
		}

		started = append(started, e)

		wg.Add(1)
		safe.Go("service "+e.svc.Name(), func() {
			defer wg.Done()
			g.supervise(ctx, e, acquired)
		})
	}

	<-ctx.Done()
	g.stop(started, &wg)
	return nil
}

// supervise keeps one service alive for as long as its policy says to.
//
// acquired says whether it already has its hardware. One that never got it starts by asking again
// rather than by being run, so a device that turns up a second late is not lost for the whole run.
func (g *Group) supervise(ctx context.Context, e *entry, acquired bool) {
	wait := e.policy.backoff

	if !acquired {
		if !g.reacquire(ctx, e, &wait) {
			return
		}
		slog.Info("service acquired", "service", e.svc.Name())
	}

	for {
		e.set(StateRunning, nil)
		began := time.Now()

		err := run(ctx, e.svc)
		ran := time.Since(began)

		if ctx.Err() != nil {
			e.set(StateStopped, nil)
			return
		}
		if err == nil {
			// Finished on its own, which is what a one-shot service does.
			e.set(StateStopped, nil)
			slog.Info("service finished", "service", e.svc.Name(), "ran", ran.Round(time.Millisecond))
			return
		}
		if !e.policy.restart {
			e.set(StateFailed, err)
			slog.Error("service failed", "service", e.svc.Name(), "err", err)
			return
		}

		// A service that stayed up a good while is having a bad moment rather than a bad life, so it
		// starts again from the shortest delay.
		if ran >= e.policy.steady {
			wait = e.policy.backoff
		}

		e.set(StateRetrying, err)
		slog.Error("service failed, restarting", "service", e.svc.Name(),
			"err", err, "ran", ran.Round(time.Millisecond), "in", wait)

		if !g.reacquire(ctx, e, &wait) {
			return
		}
		e.countRestart()
		slog.Info("service restarted", "service", e.svc.Name(), "restarts", e.status().Restarts)
	}
}

// reacquire keeps asking for a service's hardware until it has it, and says whether it got there
// before the group was told to stop.
//
// Acquired before it is run, however many attempts that takes. Running one that could not take its
// device is a service reporting itself up while doing nothing, and a Run that waits on hardware it
// never got waits forever — never returning, so never restarted.
func (g *Group) reacquire(ctx context.Context, e *entry, wait *time.Duration) bool {
	for {
		select {
		case <-ctx.Done():
			e.set(StateStopped, nil)
			return false
		case <-time.After(*wait):
		}
		*wait = grow(*wait, e.policy.maxBackoff)

		// Let go of whatever it was holding before asking for it again: the point of a restart is
		// usually to re-acquire a device, and holding the old handle would refuse it. What a
		// restart lets go of is the service's to say, because it is not always everything.
		e.release()

		err := e.start(ctx)
		if err == nil {
			return true
		}

		e.set(StateRetrying, err)
		slog.Error("service could not be reacquired", "service", e.svc.Name(), "err", err, "in", *wait)
	}
}

// grow is the next backoff, capped.
func grow(wait, max time.Duration) time.Duration {
	if next := wait * 2; next <= max {
		return next
	}
	return max
}

// stop waits for the running services to return, then closes them in reverse order.
func (g *Group) stop(started []*entry, wg *sync.WaitGroup) {
	g.mu.Lock()
	g.stopping = true
	g.mu.Unlock()

	wg.Wait()
	for i := len(started) - 1; i >= 0; i-- {
		started[i].close()
	}
}

// run calls a service and turns a panic into an error, so the restart policy covers it. Recovering
// any further out would take the loop above with it, and the service would be gone for the life of
// the process rather than restarted.
func run(ctx context.Context, svc Service) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("recovered from a panic",
				"in", "service "+svc.Name(), "panic", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return svc.Run(ctx)
}

// Start and Close stay optional at this level too, which is what lets a component implement only
// the halves it has.
func (e *entry) start(ctx context.Context) error {
	if s, ok := e.svc.(Starter); ok {
		return s.Start(ctx)
	}
	return nil
}

func (e *entry) close() {
	c, ok := e.svc.(Closer)
	if !ok {
		return
	}
	if err := c.Close(); err != nil {
		slog.Error("closing a service failed", "service", e.svc.Name(), "err", err)
	}
}

// release is what a restart does instead of closing, for a service that says the two differ.
func (e *entry) release() {
	r, ok := e.svc.(Reacquirer)
	if !ok {
		e.close()
		return
	}
	if err := r.Release(); err != nil {
		slog.Error("releasing a service failed", "service", e.svc.Name(), "err", err)
	}
}

func (e *entry) set(s State, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state, e.err, e.since = s, err, time.Now()
}

func (e *entry) countRestart() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.starts++
}

func (e *entry) status() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	return Status{Name: e.svc.Name(), State: e.state, Err: e.err, Restarts: e.starts, Since: e.since}
}
