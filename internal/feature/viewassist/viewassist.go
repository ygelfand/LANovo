package viewassist

import (
	"fmt"
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"
	sharedmessage "github.com/ygelfand/libcountertop/pkg/display/message"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/lib/viewassist"
)

func init() {
	component.Register(component.Device, Get, component.Order(70))
}

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

func (s *Satellite) View() viewassist.View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view
}

func (s *Satellite) State() viewassist.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.said
}

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

func (s *Satellite) SetState(said viewassist.State) {
	s.mu.Lock()
	s.said = said
	s.mu.Unlock()

	if said.Empty() {
		message.Get().Hide()
		return
	}

	message.Get().Show(sharedmessage.Message{
		Title: said.Title,
		Body:  said.Message,
		Tone:  sharedmessage.ToneInfo,
	}, sharedmessage.Hold(0))
}

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
