// Package message is what Home Assistant can put on the screen: a doorbell, a reminder, a warning
// that something needs attention.
//
// An action rather than an entity, because it takes arguments and says nothing about itself
// between calls.
package message

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	"github.com/ygelfand/LANovo/internal/lib/hook"
)

func init() {
	component.Register(component.Device, Get, component.Order(40))
}

// How long a message stays up when the caller does not say.
const (
	DefaultSeconds = 15
	MaxSeconds     = 3600
)

type Messages struct {
	Changed hook.Hook[bool]

	mu      sync.Mutex
	timer   *time.Timer
	current Message
	took    int
}

func (m *Messages) Took(id int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.took == id+1
}

func (m *Messages) Current() (Message, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current, m.timer != nil
}

var (
	once   sync.Once
	shared *Messages
)

func Get() *Messages {
	once.Do(func() {
		shared = &Messages{}

		// A message is the one thing on screen a person is meant to answer, so touching it is how
		// it goes away without waiting out its time.
		touch.Get().Contacts.Listen(func(c touch.Contact) {
			if c.Phase == touch.Down {
				shared.dismiss(c.ID)
			}
		})
	})
	return shared
}

// dismiss takes a message away because someone touched it, which only means anything while one is
// up.
func (m *Messages) dismiss(id int) {
	m.mu.Lock()
	showing := m.timer != nil
	m.took = 0
	if showing {
		m.took = id + 1
	}
	m.mu.Unlock()

	if !showing {
		return
	}
	slog.Info("message dismissed")
	m.Hide()
}

func (m *Messages) Name() string { return "messages" }

// Actions is what Home Assistant may call.
func (m *Messages) Actions() []*esphome.Action {
	return []*esphome.Action{
		{
			Name: "show_message",
			Args: []esphome.Arg{
				{Name: "body", Type: esphome.ArgString},
				{Name: "title", Type: esphome.ArgString},
				{Name: "tone", Type: esphome.ArgString},
				{Name: "seconds", Type: esphome.ArgInt},
			},
			Run: func(c esphome.Call) (any, error) {
				body := c.String("body")
				if body == "" {
					return nil, fmt.Errorf("show_message needs a body")
				}

				m.Show(Message{
					Title: c.String("title"),
					Body:  body,
					Tone:  tone(c.String("tone")),
				}, Hold(c.Int("seconds")))
				return nil, nil
			},
		},
		{
			Name: "clear_message",
			Run:  func(esphome.Call) (any, error) { m.Hide(); return nil, nil },
		},
	}
}

// Show puts a message up for a while. A second message replaces the first rather than queueing:
// what is worth showing is what just happened.
func (m *Messages) Show(msg Message, hold time.Duration) {
	m.mu.Lock()
	m.current = msg
	if m.timer == nil {
		m.timer = time.AfterFunc(hold, m.Hide)
	} else {
		m.timer.Reset(hold)
	}
	m.mu.Unlock()
	slog.Info("message", "title", msg.Title, "tone", msg.Tone, "for", hold)
	m.Changed.Emit(true)
}

func (m *Messages) Hide() {
	m.mu.Lock()
	if m.timer == nil {
		m.mu.Unlock()
		return
	}
	m.timer.Stop()
	m.timer = nil
	m.mu.Unlock()
	m.Changed.Emit(false)
}

// tone reads what the caller asked for, treating anything unrecognized as ordinary rather than
// refusing the message.
func tone(name string) Tone {
	for _, t := range Tones() {
		if string(t) == name {
			return t
		}
	}
	return ToneInfo
}

// Hold bounds how long a message may keep the screen, since it outranks everything the device
// shows on its own. Zero or less is the default, and anything past the ceiling is the ceiling.
func Hold(v int) time.Duration {
	switch {
	case v <= 0:
		v = DefaultSeconds
	case v > MaxSeconds:
		v = MaxSeconds
	}
	return time.Duration(v) * time.Second
}
