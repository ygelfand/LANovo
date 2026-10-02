package ble

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Hardware, Get, component.Order(30),
		component.Supervise(service.Restart(2*time.Second, time.Minute)))
}

// Radio is the Bluetooth controller, brought up and held open.
//
// It does not scan: one antenna serves wifi too, and listening costs airtime whether or not
// anybody wants the results. Whoever wants advertisements runs the scan.
type Radio struct {
	mu   sync.Mutex
	port *Port

	// What the chip and controller said about themselves, for diagnostics.
	version Version
	local   Local
	can     Features

	// sess is the one reader on the line. Everything that wants the controller goes through it, so
	// a scan and a link run at the same time instead of excluding each other.
	sess *Session

	// stop ends that reader. It outlives the call that started it, so it is not a request context.
	stop context.CancelFunc

	err error
}

var (
	once   sync.Once
	shared *Radio
)

// Get is the radio. Nothing here touches hardware: Start does.
func Get() *Radio { once.Do(func() { shared = &Radio{} }); return shared }

func (r *Radio) Name() string { return "bluetooth" }

// Start powers the chip from a known state and downloads its firmware.
//
// Always from blocked: a chip already running the firmware does not answer the loader, so a
// bring-up onto a live controller reads like a chip that is not there.
func (r *Radio) Start(context.Context) error {
	if _, err := Power(false); err != nil {
		return r.failed(err)
	}

	p, v, err := BringUp()
	if err != nil {
		return r.failed(err)
	}

	local, err := ReadLocal(p)
	if err != nil {
		p.Close()
		return r.failed(err)
	}

	can, err := ReadFeatures(p)
	if err != nil {
		p.Close()
		return r.failed(err)
	}

	// The reader starts only now. Bring-up answers a strict sequence with the loader's own framing,
	// and a demultiplexer in the middle of that is a hazard rather than a help.
	ctx, stop := context.WithCancel(context.Background())
	sess := Reader(ctx, p)

	r.mu.Lock()
	r.port, r.version, r.local, r.can, r.err = p, v, local, can, nil
	r.sess, r.stop = sess, stop
	r.mu.Unlock()
	return nil
}

// Close gives the chip back and leaves the radio off, since nothing is holding it and Start
// powers up from blocked anyway.
func (r *Radio) Close() error {
	r.mu.Lock()
	p, stop := r.port, r.stop
	r.port, r.sess, r.stop = nil, nil, nil
	r.mu.Unlock()

	// The reader before the port: a read that fails because the file went away is a fault to report
	// rather than the shutdown it is.
	if stop != nil {
		stop()
	}
	if p != nil {
		p.Close()
	}
	_, err := Power(false)
	return err
}

// Startup is for the boot screen. The device manages without Bluetooth, so a failure marks the row
// rather than holding the boot.
func (r *Radio) Startup() component.Progress {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch {
	case r.err != nil:
		return component.Progress{Failed: true, Doing: r.err.Error()}
	case r.port == nil:
		return component.Progress{Doing: "loading the firmware"}
	}
	if r.sess != nil {
		if err := r.sess.Err(); err != nil {
			return component.Progress{Failed: true, Doing: err.Error()}
		}
	}
	return component.Progress{Done: true}
}

// Up reports whether the controller is running its firmware and holding the line.
//
// A reader that has stopped counts as down. The port is still open at that point and nothing else
// says otherwise, so without this a chip whose line failed reads as working right up until every
// command times out.
func (r *Radio) Up() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.port != nil && r.sess != nil && r.sess.Err() == nil
}

// Can is what the controller supports, which decides what is worth asking of it.
func (r *Radio) Can() Features {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.can
}

// Controller is what it says it is, for diagnostics.
func (r *Radio) Controller() (Version, Local) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.version, r.local
}

// Ask sends one command on the controller's own channel and waits for its answer.
//
// For asking the chip something the stack has no opinion about, which is how an opcode gets tried
// against a controller that may or may not know it: an unknown one is refused, and a refusal is an
// answer.
func (r *Radio) Ask(cmd []byte, within time.Duration) (byte, []byte, error) {
	s, err := r.Line()
	if err != nil {
		return 0, nil, err
	}

	e, err := s.Ask(cmd, within)
	if err != nil {
		return 0, nil, err
	}
	return e.Code, e.Params, nil
}

// Sniff reports everything that arrives on the line for a while, for telling a chip that said
// nothing from one whose answer nobody was waiting for.
func (r *Radio) Sniff(within time.Duration) ([]string, error) {
	s, err := r.Line()
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	p := r.port
	r.mu.Unlock()

	return append([]string{fmt.Sprintf("the chip is marked %s",
		map[bool]string{true: "asleep", false: "awake"}[p.Asleep()])},
		s.Sniff(within)...), nil
}

// Line is the reader, or why there is not one.
func (r *Radio) Line() (*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sess == nil {
		return nil, fmt.Errorf("ble: the radio is not up")
	}
	if err := r.sess.Err(); err != nil {
		return nil, err
	}
	return r.sess, nil
}

// Scan listens for advertisements until ctx ends.
func (r *Radio) Scan(ctx context.Context, active bool, found func(Advertisement)) error {
	s, err := r.Line()
	if err != nil {
		return err
	}
	return Scan(ctx, s, active, found)
}

// Speaker makes the controller findable under a name and carries what connects to it until ctx
// ends.
func (r *Radio) Speaker(ctx context.Context, name string, on Classic) error {
	s, err := r.Line()
	if err != nil {
		return err
	}

	if err := discoverable(s, name, true); err != nil {
		return err
	}

	// Invisible again however this ends. A device left discoverable by a service that stopped is
	// one a phone offers to connect to and then cannot.
	defer func() {
		if err := discoverable(s, name, false); err != nil {
			slog.Warn("could not stop being discoverable", "err", err)
		}
	}()

	return Serve(ctx, s, on)
}

// failed records why the bring-up did not work, so the boot screen can say.
func (r *Radio) failed(err error) error {
	r.mu.Lock()
	r.err = err
	r.mu.Unlock()
	return err
}
