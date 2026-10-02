package shell

import (
	"slices"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/lib/hook"
)

func init() {
	component.Register(component.Device, Get, component.Order(34))
}

const (
	DockTimeout     = 15 * time.Second
	SettingsTimeout = 45 * time.Second

	settling = 400 * time.Millisecond
)

type View interface {
	Covers() bool
}

type Timeouter interface {
	Timeout() time.Duration
}

type Sleeper interface{ Asleep() bool }

type Waker interface{ Wakes() bool }

type Shell struct {
	mu     sync.Mutex
	stack  []View
	holds  map[View]bool
	timer  *time.Timer
	closed time.Time

	Changed hook.Hook[Change]
	Redrawn hook.Hook[struct{}]
}

type Change struct{ From, To View }

type Stacked struct {
	View View
	Held bool
}

var (
	once   sync.Once
	shared *Shell
)

func Get() *Shell {
	once.Do(func() { shared = &Shell{} })
	return shared
}

func (s *Shell) Name() string { return "shell" }

func (s *Shell) Open() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.stack) > 0
}

func (s *Shell) Closing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.closed) < settling
}

func (s *Shell) Top() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.top()
}

func (s *Shell) top() View {
	if n := len(s.stack); n > 0 {
		return s.stack[n-1]
	}
	return nil
}

func (s *Shell) Visible(v View) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Contains(Showing(s.stack), v)
}

func Showing(stack []View) []View {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].Covers() {
			return stack[i:]
		}
	}
	return stack
}

func (s *Shell) Stack() []Stacked {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Stacked, len(s.stack))
	for i, v := range s.stack {
		out[i] = Stacked{View: v, Held: s.holds[v]}
	}
	return out
}

func (s *Shell) Views() []View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.stack)
}

func (s *Shell) changed(from, to View) {
	if from != to {
		s.Changed.Emit(Change{From: from, To: to})
	}
	s.Redraw()
}

func (s *Shell) refuses(v View) bool {
	n := len(s.stack)
	if n == 0 || s.stack[n-1] == v {
		return false
	}
	if sl, ok := s.stack[n-1].(Sleeper); !ok || !sl.Asleep() {
		return false
	}
	if !v.Covers() {
		return false
	}
	w, ok := v.(Waker)
	return !ok || !w.Wakes()
}

func (s *Shell) Push(v View) {
	s.mu.Lock()
	if s.refuses(v) {
		s.mu.Unlock()
		return
	}
	from := s.top()
	if !slices.Contains(s.stack, v) {
		s.stack = append(s.stack, v)
	}
	to := s.top()
	s.mu.Unlock()
	s.changed(from, to)
	s.wake()
}

func (s *Shell) Hold(v View) *Hold {
	s.mu.Lock()
	if s.refuses(v) {
		s.mu.Unlock()
		return nil
	}
	if s.holds == nil {
		s.holds = map[View]bool{}
	}
	s.holds[v] = true
	s.mu.Unlock()
	s.Push(v)
	return &Hold{shell: s, view: v}
}

type Hold struct {
	shell *Shell
	view  View
}

func (h *Hold) Keep(on bool) {
	if h == nil {
		return
	}
	h.shell.mu.Lock()
	if on {
		if h.shell.holds == nil {
			h.shell.holds = map[View]bool{}
		}
		h.shell.holds[h.view] = true
	} else {
		delete(h.shell.holds, h.view)
	}
	h.shell.mu.Unlock()
	h.shell.wake()
}

func (h *Hold) Release() {
	if h == nil {
		return
	}
	h.shell.Remove(h.view)
}

func (h *Hold) Held() bool {
	if h == nil {
		return false
	}
	h.shell.mu.Lock()
	defer h.shell.mu.Unlock()
	return h.shell.holds[h.view]
}

func (s *Shell) Holding(v View) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.holds[v]
}

func (s *Shell) Remove(v View) {
	s.mu.Lock()
	from := s.top()
	s.stack = slices.DeleteFunc(s.stack, func(have View) bool { return have == v })
	delete(s.holds, v)
	empty := len(s.stack) == 0
	to := s.top()
	s.mu.Unlock()
	s.changed(from, to)
	if empty {
		s.Close()
	}
}

func (s *Shell) Pop() {
	s.mu.Lock()
	gone := s.top()
	s.mu.Unlock()
	if gone != nil {
		s.Remove(gone)
	}
}

func (s *Shell) Close() {
	s.mu.Lock()
	from := s.top()
	timer := s.timer
	s.closed = time.Now()
	s.stack, s.timer = nil, nil
	clear(s.holds)
	s.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
	s.changed(from, nil)
}

func (s *Shell) Redraw() { s.Redrawn.Emit(struct{}{}) }

func (s *Shell) Touch() {
	if s.Open() {
		s.wake()
	}
}

func (s *Shell) idled() {
	s.mu.Lock()
	from := s.top()
	s.stack = slices.DeleteFunc(s.stack, func(v View) bool { return !s.holds[v] })
	empty := len(s.stack) == 0
	to := s.top()
	s.mu.Unlock()
	s.changed(from, to)
	if empty {
		s.Close()
	}
}

func (s *Shell) wake() {
	s.mu.Lock()
	defer s.mu.Unlock()
	idle := s.timeout()
	if s.timer == nil {
		s.timer = time.AfterFunc(idle, s.idled)
		return
	}
	s.timer.Reset(idle)
}

func (s *Shell) timeout() time.Duration {
	var idle time.Duration
	for _, v := range s.stack {
		if t, ok := v.(Timeouter); ok {
			idle = max(idle, t.Timeout())
		}
	}
	if idle == 0 {
		return DockTimeout
	}
	return idle
}
