// Package viewassist answers View Assist's actions, so an automation can drive this screen with no
// custom integration installed.
//
// The additive floor of #54. What View Assist asks a satellite to do is navigate to one of a small
// set of views and put a title and a message on screen; the second of those is the shape the
// message overlay already has, so it is wired to it. Navigation is recorded and reported, and the
// views themselves are drawn later — a satellite that accepts a navigation and shows nothing is
// still one an automation can be written against, and refusing it would be worse.
//
// The contract itself is internal/lib/viewassist. This is the wiring: actions one side, the screen
// the other.
package viewassist

import (
	"fmt"
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/lib/viewassist"
)

func init() {
	component.Register(component.Device, Get, component.Order(70))
}

// Satellite is what View Assist is talking to.
type Satellite struct {
	mu   sync.Mutex
	view viewassist.View
	said viewassist.State
}

var (
	once   sync.Once
	shared *Satellite
)

func Get() *Satellite {
	once.Do(func() { shared = &Satellite{view: viewassist.Clock} })
	return shared
}

func (s *Satellite) Name() string { return "view assist" }

// View is the view last navigated to.
func (s *Satellite) View() viewassist.View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view
}

// State is the title and message last set.
func (s *Satellite) State() viewassist.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.said
}

// Navigate takes the satellite to the view a path names.
//
// A path that names no view is refused rather than ignored, because View Assist's own status icons
// carry external paths and a satellite that silently accepted one would look like it had obeyed. A
// webpage is refused for the same reason and a different cause: it is markup, and there is no
// browser here.
func (s *Satellite) Navigate(path string) error {
	v, ok := viewassist.Navigated(path)
	if !ok {
		return fmt.Errorf("navigate: %q names no view", path)
	}
	if !v.Drawable() {
		return fmt.Errorf("navigate: %q needs a browser and this device has none", v)
	}

	s.mu.Lock()
	s.view = v
	s.mu.Unlock()

	slog.Debug("view assist navigated", "view", v, "path", path)
	return nil
}

// SetState puts a title and a message on screen, and takes them down when both are empty.
//
// Onto the message overlay, which is the same shape: a title, a body, and a while to stay up.
func (s *Satellite) SetState(said viewassist.State) {
	s.mu.Lock()
	s.said = said
	s.mu.Unlock()

	if said.Empty() {
		message.Get().Hide()
		return
	}

	message.Get().Show(message.Message{
		Title: said.Title,
		Body:  said.Message,
		Tone:  message.ToneInfo,
	}, message.Hold(0))
}

// Actions is what Home Assistant may call. The names are View Assist's own, so an automation
// written for a satellite reaches this one without being rewritten.
func (s *Satellite) Actions() []*esphome.Action {
	return []*esphome.Action{
		{
			Name: viewassist.Navigate,
			Args: []esphome.Arg{{Name: "path", Type: esphome.ArgString}},
			Run: func(c esphome.Call) (any, error) {
				return nil, s.Navigate(c.String("path"))
			},
		},
		{
			Name: viewassist.SetState,
			Args: []esphome.Arg{
				{Name: "title", Type: esphome.ArgString},
				{Name: "message", Type: esphome.ArgString},
			},
			Run: func(c esphome.Call) (any, error) {
				s.SetState(viewassist.State{
					Title:   c.String("title"),
					Message: c.String("message"),
				})
				return nil, nil
			},
		},
	}
}
