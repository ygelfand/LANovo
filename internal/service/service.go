// Package service supervises the parts of lanovod that have a life of their own.
//
// lanovod is a handful of long-running loops over hardware Android can take back: the panel, the
// speaker, the microphones, the radio, the API server. Each can fail on its own, and a bare
// goroutine that returns takes its subsystem with it for the rest of the process.
//
// A Group starts them in order, restarts what breaks, and can say what is broken, which is the part
// that matters on a device nobody is looking at.
package service

import (
	"context"
	"time"
)

// Service is a loop with a name.
type Service interface {
	// Name is how it appears in logs and diagnostics.
	Name() string

	// Run holds the service until ctx is canceled. Returning nil means it finished or stopped
	// cleanly; returning an error means it broke, and a restartable service is started again.
	Run(ctx context.Context) error
}

// Starter acquires whatever the service needs. It runs before Run, and again on every restart:
// re-acquiring is usually the whole point, since the device it wants may have been taken while it
// was gone.
type Starter interface {
	Start(ctx context.Context) error
}

// Closer releases it again, after Run has returned and when the group is stopping.
type Closer interface {
	Close() error
}

// Reacquirer is a service whose restart has to let go of less than its shutdown does.
//
// A restart calls Release where a stop calls Close, and the two differ wherever Close gives up
// something the device cannot simply take back. The DHCP client is why this exists: its Close
// tells the server the address is free and strips it from the interface, which is right when the
// device is stopping and is how a restart puts it off the network — and off it for good if the
// service then cannot come back.
//
// A service that implements both gets Release on a restart and Close on a stop. One that
// implements only Closer gets Close for both, which is the right default: letting go of a device
// is usually the whole point of restarting.
type Reacquirer interface {
	Release() error
}

// State is where a service has got to.
type State string

const (
	// StateWaiting is registered but not yet started.
	StateWaiting State = "waiting"

	// StateRunning is up.
	StateRunning State = "running"

	// StateRetrying is down and about to be started again.
	StateRetrying State = "retrying"

	// StateFailed is down and not coming back: either it is not restartable, or it could not be
	// acquired and was optional.
	StateFailed State = "failed"

	// StateStopped is down because it was asked to stop.
	StateStopped State = "stopped"
)

// Status is what a service is doing, for diagnostics.
type Status struct {
	Name     string
	State    State
	Err      error
	Restarts int

	// Since is when it entered this state.
	Since time.Time
}

// Healthy reports whether everything that was meant to be running is.
func Healthy(all []Status) bool {
	for _, s := range all {
		if s.State == StateFailed || s.State == StateRetrying {
			return false
		}
	}
	return true
}

// policy is how a Group treats one service.
type policy struct {
	// required means the process cannot do its job without it: failing to acquire it is fatal.
	required bool

	// restart means Run returning an error should be followed by starting it again.
	restart bool

	backoff    time.Duration
	maxBackoff time.Duration

	// steady is how long a service must stay up before its backoff is forgotten, so a service that
	// breaks once an hour does not creep towards the maximum delay.
	steady time.Duration
}

// Option adjusts how a service is supervised.
type Option func(*policy)

// Required marks a service the process cannot run without. If it cannot be acquired, Run gives up
// and lanovod exits, which is the right answer when init will start it again in a moment.
func Required() Option {
	return func(p *policy) { p.required = true }
}

// Restart keeps a service alive: when it breaks it is closed, acquired again and run again, waiting
// a little longer each time up to max.
func Restart(initial, max time.Duration) Option {
	return func(p *policy) {
		p.restart = true
		p.backoff = initial
		p.maxBackoff = max
	}
}

// Once marks a service that is expected to finish, such as the boot screen. Returning nil leaves it
// stopped rather than looking like something that died.
func Once() Option {
	return func(p *policy) { p.restart = false }
}

func defaults() policy {
	return policy{
		backoff:    time.Second,
		maxBackoff: 30 * time.Second,
		steady:     time.Minute,
	}
}
