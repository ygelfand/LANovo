// Package states is what Home Assistant knows, for the parts of the device that want to show it.
//
// The device cannot go and read a state. Home Assistant asks, once per connection, what this
// device wants to be told about; the device answers with a list; and from then on every change to
// one of those arrives unprompted. So a follower is registered before anything connects, and the
// list is offered again on every connection.
//
// This is the same route ESPHome's own `homeassistant` platforms take, which is why a value shows
// up without anything being configured on the server: the integration honors the list a device
// sends it.
package states

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"
	"google.golang.org/protobuf/proto"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/lib/hook"
)

func init() {
	component.Register(component.Device, Get, component.Order(5))
}

// States is everything being followed.
type States struct {
	mu        sync.Mutex
	following []*Value
	to        sender
}

// sender is all this package needs a connection to do, so a test can be one. *esphome.Conn is the
// only implementation on the device.
type sender interface {
	Send(proto.Message) error
}

// wrap is a connection as a sender, and nothing at all when there is no connection. A nil pointer
// put in an interface is not a nil interface, and this is the one place that can go wrong.
func wrap(conn *esphome.Conn) sender {
	if conn == nil {
		return nil
	}
	return conn
}

// Value is one entity's state, or one attribute of it, as Home Assistant last reported it.
type Value struct {
	// Entity is the entity_id, and Attribute is empty for the state itself.
	Entity    string
	Attribute string

	// Changed fires when the value is not what it was. Home Assistant reports on change, but it
	// also reports the current value whenever a connection is made, and a redraw for a value that
	// has not moved is a redraw for nothing.
	Changed hook.Hook[string]

	mu    sync.Mutex
	state string
	known bool
}

var (
	once   sync.Once
	shared *States
)

func Get() *States {
	once.Do(func() { shared = &States{} })
	return shared
}

func (s *States) Name() string { return "states" }

// Follow asks to be told about an entity, or about one attribute of it.
//
// Following the same thing twice gives the same Value back, so two features showing one
// temperature ask Home Assistant for it once and cannot disagree about it.
//
// Usually called while a feature is being built, which is before anything has connected. One
// asked for later is offered to whatever is connected now, so a value chosen in the settings does
// not wait for a reconnection.
func (s *States) Follow(entity, attribute string) *Value {
	entity = strings.TrimSpace(entity)
	if entity == "" {
		return &Value{}
	}

	s.mu.Lock()
	for _, v := range s.following {
		if v.Entity == entity && v.Attribute == attribute {
			s.mu.Unlock()
			return v
		}
	}

	v := &Value{Entity: entity, Attribute: attribute}
	s.following = append(s.following, v)
	to := s.to
	s.mu.Unlock()

	ask(to, v)
	return v
}

// Following is everything on the list, for diagnostics and tests.
func (s *States) Following() []*Value {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*Value(nil), s.following...)
}

// Handle answers the question and takes the answers.
func (s *States) Handle(_ context.Context, conn *esphome.Conn, msg proto.Message) error {
	switch m := msg.(type) {
	case *api.SubscribeHomeAssistantStatesRequest:
		s.offer(wrap(conn))
	case *api.HomeAssistantStateResponse:
		s.take(m.GetEntityId(), m.GetAttribute(), m.GetState())
	}
	return nil
}

// offer sends the whole list. Home Assistant asks once per connection and acts on what it is sent,
// so everything being followed goes out together.
func (s *States) offer(to sender) {
	s.mu.Lock()
	s.to = to
	list := append([]*Value(nil), s.following...)
	s.mu.Unlock()

	for _, v := range list {
		ask(to, v)
	}
	slog.Debug("asked Home Assistant to report on", "entities", len(list))
}

// ask puts one entity on the list.
func ask(to sender, v *Value) {
	if to == nil || v.Entity == "" {
		return
	}

	// once is false: this is a value shown on a screen that stays on, not a reading taken at
	// start-up.
	err := to.Send(&api.SubscribeHomeAssistantStateResponse{
		EntityId:  v.Entity,
		Attribute: v.Attribute,
		Once:      false,
	})
	if err != nil {
		slog.Debug("asking for a state failed", "entity", v.Entity, "err", err)
	}
}

// take records what Home Assistant reported.
func (s *States) take(entity, attribute, state string) {
	s.mu.Lock()
	list := append([]*Value(nil), s.following...)
	s.mu.Unlock()

	for _, v := range list {
		if v.Entity == entity && v.Attribute == attribute {
			v.set(state)
			return
		}
	}
	slog.Debug("a state arrived for something nothing is following", "entity", entity, "attribute", attribute)
}

// Get is what was last reported, and whether anything has been.
func (v *Value) Get() (string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.state, v.known
}

// Known reports whether Home Assistant has said anything about this yet.
func (v *Value) Known() bool {
	_, ok := v.Get()
	return ok
}

// Float is the value as a number. Home Assistant sends every state as a string, including the
// ones that are plainly numbers, and an entity that is unavailable sends "unavailable" rather
// than nothing at all.
func (v *Value) Float() (float64, bool) {
	s, ok := v.Get()
	if !ok {
		return 0, false
	}

	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// set records a report and tells whoever is listening, if it is news.
func (v *Value) set(state string) {
	v.mu.Lock()
	same := v.known && v.state == state
	v.state, v.known = state, true
	v.mu.Unlock()

	if !same {
		v.Changed.Emit(state)
	}
}
