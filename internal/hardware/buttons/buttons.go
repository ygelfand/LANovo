// Package buttons reads the physical controls.
//
// None are kernel input devices — getevent sees only the power key, touchscreen and codec jacks.
// The volume keys, mic mute slider and camera shutter are plain GPIO lines with edge=both, so
// POLLPRI on the sysfs value delivers each change. This works even while another process holds
// the line.
//
// One press usually means several things — the device acts on it, Home Assistant hears about it —
// so it is a hook rather than a callback. Listeners run on the reader goroutine and must not block.
package buttons

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/service"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

func init() {
	component.Register(component.Hardware, Get, component.Order(10),
		component.Supervise(service.Restart(time.Second, 30*time.Second)))
}

// Controller reads the buttons for the life of the process.
type Controller struct {
	// Events carries every change to whoever is listening.
	Events hook.Hook[Event]

	mu    sync.Mutex
	watch *Watcher
}

var (
	once   sync.Once
	shared *Controller
)

// Get is the buttons. Nothing here opens anything: Start does.
func Get() *Controller { once.Do(func() { shared = &Controller{} }); return shared }

func (c *Controller) Name() string { return "buttons" }

// Deliver reports a change as though a line had moved, for a caller standing in for a finger.
//
// The same hook the reader emits on, so everything downstream cannot tell the difference — which
// is the point. A harness that called what the buttons happen to do today would be testing its own
// copy of the wiring.
func (c *Controller) Deliver(e Event) { c.Events.Emit(e) }

// All is every control the driver knows, for anything that offers them by name.
func All() []Button {
	out := make([]Button, 0, len(lines()))
	for _, l := range lines() {
		out = append(out, l.button)
	}
	return out
}

// Startup is ready once the lines are open.
func (c *Controller) Startup() component.Progress {
	c.mu.Lock()
	w := c.watch
	c.mu.Unlock()

	if w == nil {
		return component.Progress{Doing: "taking the buttons"}
	}

	// Silent when they are all there, which is the usual case and not worth a line of its own.
	// A board missing one is what someone reading this wants to know.
	if missing := len(lines()) - len(w.files); missing > 0 {
		return component.Progress{
			Done:  true,
			Doing: fmt.Sprintf("%d of %d, the rest could not be read", len(w.files), len(lines())),
		}
	}
	return component.Progress{Done: true}
}

// Start exports the lines and opens them. Finding none is an error: the buttons are the one part of
// the device that should work whatever else is wrong, so silently having none is worth a restart.
func (c *Controller) Start(context.Context) error {
	w, problems := Watch()
	for _, p := range problems {
		// A line that cannot be read is reported and skipped: a board without one of these should
		// still have the others.
		fmt.Fprintln(os.Stderr, "buttons:", p)
	}
	if len(w.files) == 0 {
		w.Close()
		return errors.New("buttons: no gpio lines could be opened")
	}

	c.mu.Lock()
	c.watch = w
	c.mu.Unlock()
	return nil
}

// Run reads until ctx is canceled, or until a line fails. A failure is returned rather than logged
// so the supervisor reopens the lines: a reader that has quietly exited leaves the device with dead
// buttons.
func (c *Controller) Run(ctx context.Context) error {
	c.mu.Lock()
	w := c.watch
	c.mu.Unlock()

	if w == nil {
		return errors.New("buttons: not started")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		events, err := w.Next(500)
		if err != nil {
			return err
		}
		for _, e := range events {
			c.Events.Emit(e)
		}
	}
}

// Close releases the lines, which is also what unblocks the reader.
func (c *Controller) Close() error {
	c.mu.Lock()
	w := c.watch
	c.watch = nil
	c.mu.Unlock()

	if w != nil {
		w.Close()
	}
	return nil
}

// State is what a control reads right now, and whether it could be read at all.
func (c *Controller) State(b Button) (bool, bool) {
	c.mu.Lock()
	w := c.watch
	c.mu.Unlock()

	if w == nil {
		return false, false
	}
	return w.State(b)
}

// Button is one control.
type Button string

const (
	VolumeUp    Button = "volume up"
	VolumeDown  Button = "volume down"
	MicMute     Button = "mic mute"
	CameraCover Button = "camera shutter"
)

// line is a control and the GPIO behind it.
type line struct {
	button Button
	gpio   int

	// activeLow inverts the reading. Buttons and the mic slider read 1 when engaged; the camera
	// shutter reads 1 when OPEN.
	activeLow bool
}

// lines is this board's controls.
func lines() []line {
	var out []line
	for _, b := range board.Current().Buttons {
		out = append(out, line{Button(b.Control), b.GPIO, b.ActiveLow})
	}
	return out
}

// Event is a change on one control. Pressed is true when a button is held or a slider is engaged —
// for the camera that means the shutter is closed.
type Event struct {
	Button  Button
	Pressed bool
}

// Watcher reports changes on every control at once.
type Watcher struct {
	files []*os.File
	pfd   []pollFd
	lines []line
	last  []bool
}

// syscall does not wrap poll(2) on Linux. 168 is __NR_poll on arm EABI.
const sysPoll = 168

const (
	pollPri = 0x0002
	pollErr = 0x0008
)

type pollFd struct {
	fd      int32
	events  int16
	revents int16
}

// export makes a line readable. Lenovo's OEM app exported these; with it gone, nothing does.
func export(gpio int) error {
	dir := fmt.Sprintf("/sys/class/gpio/gpio%d", gpio)
	if _, err := os.Stat(dir); err != nil {
		if err := os.WriteFile("/sys/class/gpio/export", []byte(strconv.Itoa(gpio)), 0o200); err != nil {
			// EBUSY means someone else already has it, which is fine.
			if !errors.Is(err, syscall.EBUSY) {
				return fmt.Errorf("export gpio%d: %w", gpio, err)
			}
		}
		// ueventd creates the attributes a moment after the export.
		for i := 0; i < 20; i++ {
			if _, err := os.Stat(dir); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	if err := os.WriteFile(dir+"/direction", []byte("in"), 0o200); err != nil {
		return fmt.Errorf("gpio%d direction: %w", gpio, err)
	}
	// Edges are what make this interrupt-driven rather than a polling loop.
	if err := os.WriteFile(dir+"/edge", []byte("both"), 0o200); err != nil {
		return fmt.Errorf("gpio%d edge: %w", gpio, err)
	}
	return nil
}

// Watch opens every control. Lines that cannot be read are reported and skipped.
func Watch() (*Watcher, []error) {
	w := &Watcher{}
	var problems []error

	for _, l := range lines() {
		if err := export(l.gpio); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", l.button, err))
			continue
		}

		path := fmt.Sprintf("/sys/class/gpio/gpio%d/value", l.gpio)
		f, err := os.Open(path)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s (gpio%d): %w", l.button, l.gpio, err))
			continue
		}
		w.files = append(w.files, f)
		w.lines = append(w.lines, l)
		w.pfd = append(w.pfd, pollFd{fd: int32(f.Fd()), events: pollPri | pollErr})
		w.last = append(w.last, l.read(f))
	}
	return w, problems
}

// State is what a control reads right now, without waiting for it to change.
func (w *Watcher) State(b Button) (bool, bool) {
	for i, l := range w.lines {
		if l.button == b {
			return w.last[i], true
		}
	}
	return false, false
}

// Next blocks until something changes and returns every change it found. timeoutMS below zero
// waits indefinitely; a timeout returns no events and no error.
func (w *Watcher) Next(timeoutMS int) ([]Event, error) {
	if len(w.pfd) == 0 {
		return nil, fmt.Errorf("no gpio lines could be opened")
	}

	n, _, e := syscall.Syscall(sysPoll,
		uintptr(unsafe.Pointer(&w.pfd[0])), uintptr(len(w.pfd)), uintptr(timeoutMS))
	if e == syscall.EINTR {
		return nil, nil
	}
	if e != 0 {
		return nil, fmt.Errorf("poll: %w", e)
	}
	if n == 0 {
		return nil, nil
	}

	var events []Event
	for i := range w.pfd {
		if w.pfd[i].revents == 0 {
			continue
		}
		now := w.lines[i].read(w.files[i])
		if now == w.last[i] {
			continue
		}
		w.last[i] = now
		events = append(events, Event{Button: w.lines[i].button, Pressed: now})
	}
	return events, nil
}

func (w *Watcher) Close() {
	for _, f := range w.files {
		f.Close()
	}
}

// read re-reads from the start, which clears the poll and arms the next edge.
func (l line) read(f *os.File) bool {
	var b [8]byte
	if _, err := f.Seek(0, 0); err != nil {
		return false
	}
	n, err := f.Read(b[:])
	if err != nil || n == 0 {
		return false
	}
	on := strings.TrimSpace(string(b[:n])) == "1"
	if l.activeLow {
		return !on
	}
	return on
}
