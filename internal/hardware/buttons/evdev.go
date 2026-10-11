package buttons

import (
	"context"
	"fmt"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/input"

	"github.com/ygelfand/LANovo/internal/board"
)

type evdev struct {
	mu      sync.Mutex
	devices []*input.Device
	last    map[Button]bool
}

func watchEvdev() (*evdev, []error) {
	e := &evdev{last: map[Button]bool{}}

	devices, err := input.List()
	if err != nil {
		return e, []error{fmt.Errorf("listing input devices: %w", err)}
	}
	e.devices = devices

	var problems []error
	for code, name := range board.Current().Keys {
		for _, d := range devices {
			if !d.HasKey(code) {
				continue
			}
			on, err := d.Key(code)
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", name, err))
				break
			}
			e.last[Button(name)] = on
			break
		}
	}
	return e, problems
}

func (e *evdev) State(b Button) (bool, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	on, ok := e.last[b]
	return on, ok
}

func (e *evdev) Run(ctx context.Context, emit func(Event)) error {
	failed := make(chan error, len(e.devices))
	for _, d := range e.devices {
		go func(d *input.Device) { failed <- e.watch(ctx, d, emit) }(d)
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-failed:
		return err
	}
}

func (e *evdev) watch(ctx context.Context, d *input.Device, emit func(Event)) error {
	keys := board.Current().Keys
	for {
		ev, err := d.Read()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("buttons: reading %s: %w", d.Path, err)
		}
		if ev.Type != input.EvKey || ev.Value == 2 {
			continue
		}
		name, ok := keys[ev.Code]
		if !ok {
			continue
		}

		b := Button(name)
		pressed := ev.Value == 1

		e.mu.Lock()
		if was, known := e.last[b]; known && was == pressed {
			e.mu.Unlock()
			continue
		}
		e.last[b] = pressed
		e.mu.Unlock()

		emit(Event{Button: b, Pressed: pressed})
	}
}

func (e *evdev) Close() {
	for _, d := range e.devices {
		_ = d.Close()
	}
}
