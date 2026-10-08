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

	"github.com/ygelfand/libcountertop/pkg/hook"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	component.Register(component.Hardware, Get, component.Order(10),
		component.Supervise(service.Restart(time.Second, 30*time.Second)))
}

type Controller struct {
	Events hook.Hook[Event]

	mu    sync.Mutex
	watch *Watcher
}

var (
	once   sync.Once
	shared *Controller
)

func Get() *Controller { once.Do(func() { shared = &Controller{} }); return shared }

func (c *Controller) Name() string { return "buttons" }

func (c *Controller) Deliver(e Event) { c.Events.Emit(e) }

func All() []Button {
	out := make([]Button, 0, len(lines()))
	for _, l := range lines() {
		out = append(out, l.button)
	}
	return out
}

func (c *Controller) Startup() component.Progress {
	c.mu.Lock()
	w := c.watch
	c.mu.Unlock()

	if w == nil {
		return component.Progress{Doing: "taking the buttons"}
	}

	if missing := len(lines()) - len(w.files); missing > 0 {
		return component.Progress{
			Done:  true,
			Doing: fmt.Sprintf("%d of %d, the rest could not be read", len(w.files), len(lines())),
		}
	}
	return component.Progress{Done: true}
}

func (c *Controller) Start(context.Context) error {
	w, problems := Watch()
	for _, p := range problems {
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

func (c *Controller) State(b Button) (bool, bool) {
	c.mu.Lock()
	w := c.watch
	c.mu.Unlock()

	if w == nil {
		return false, false
	}
	return w.State(b)
}

type Button string

const (
	VolumeUp    Button = "volume up"
	VolumeDown  Button = "volume down"
	MicMute     Button = "mic mute"
	CameraCover Button = "camera shutter"
)

type line struct {
	button Button
	gpio   int

	// Buttons and the mic slider read 1 when engaged; the camera shutter reads 1 when open.
	activeLow bool
}

func lines() []line {
	var out []line
	for _, b := range board.Current().Buttons {
		out = append(out, line{Button(b.Control), b.GPIO, b.ActiveLow})
	}
	return out
}

type Event struct {
	Button  Button
	Pressed bool
}

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

func export(gpio int) error {
	dir := fmt.Sprintf("/sys/class/gpio/gpio%d", gpio)
	if _, err := os.Stat(dir); err != nil {
		if err := os.WriteFile(
			"/sys/class/gpio/export",
			[]byte(strconv.Itoa(gpio)),
			0o200,
		); err != nil {
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
	if err := os.WriteFile(dir+"/edge", []byte("both"), 0o200); err != nil {
		return fmt.Errorf("gpio%d edge: %w", gpio, err)
	}
	return nil
}

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

func (w *Watcher) State(b Button) (bool, bool) {
	for i, l := range w.lines {
		if l.button == b {
			return w.last[i], true
		}
	}
	return false, false
}

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
