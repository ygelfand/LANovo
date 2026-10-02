package states

import (
	"context"
	"sync"
	"testing"

	"github.com/ygelfand/go-esphome-device/api"
	"google.golang.org/protobuf/proto"
)

// peer is Home Assistant's end, keeping what the device sent it.
type peer struct {
	mu   sync.Mutex
	sent []*api.SubscribeHomeAssistantStateResponse
	err  error
}

func (p *peer) Send(msg proto.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if m, ok := msg.(*api.SubscribeHomeAssistantStateResponse); ok {
		p.sent = append(p.sent, m)
	}
	return p.err
}

func (p *peer) asked() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := make([]string, 0, len(p.sent))
	for _, m := range p.sent {
		name := m.GetEntityId()
		if a := m.GetAttribute(); a != "" {
			name += "." + a
		}
		out = append(out, name)
	}
	return out
}

// Home Assistant asks once per connection what this device wants to hear about, and the whole
// list goes back. Nothing is followed unless it is on that list.
func TestEverythingFollowedIsOfferedWhenAsked(t *testing.T) {
	s := &States{}
	s.Follow("weather.home", "")
	s.Follow("weather.home", "temperature")
	s.Follow("sensor.back_door", "")

	p := &peer{}
	s.offer(p)

	want := []string{"weather.home", "weather.home.temperature", "sensor.back_door"}
	got := p.asked()

	if len(got) != len(want) {
		t.Fatalf("offered %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("offer %d is %q, want %q", i, got[i], want[i])
		}
	}
}

// A value the screen keeps on showing, not a reading taken once: once would have Home Assistant
// answer and stop.
func TestTheDeviceAsksToBeToldEveryTime(t *testing.T) {
	s := &States{}
	s.Follow("weather.home", "")

	p := &peer{}
	s.offer(p)

	if p.sent[0].GetOnce() {
		t.Error("the device asked to be told once")
	}
}

// Two features showing one temperature ask for it once and read the same value, or they can
// disagree about it on the same screen.
func TestFollowingTheSameThingTwiceIsOneSubscription(t *testing.T) {
	s := &States{}

	first := s.Follow("weather.home", "temperature")
	second := s.Follow("weather.home", "temperature")

	if first != second {
		t.Error("following the same thing twice gave two values")
	}
	if got := len(s.Following()); got != 1 {
		t.Errorf("%d subscriptions, want 1", got)
	}

	// And the attribute is part of what makes it the same thing.
	if s.Follow("weather.home", ""); len(s.Following()) != 2 {
		t.Error("the state and an attribute of it were taken for the same subscription")
	}
}

// Something followed after Home Assistant has already asked is offered on the connection that is
// there, rather than waiting for the next one.
func TestFollowingLateIsOfferedStraightAway(t *testing.T) {
	s := &States{}

	p := &peer{}
	s.offer(p)
	s.Follow("sensor.hallway", "")

	if got := p.asked(); len(got) != 1 || got[0] != "sensor.hallway" {
		t.Errorf("offered %v, want the one just followed", got)
	}
}

// With nothing connected there is nobody to ask, and following has to be safe anyway: features
// are built long before the server is listening.
func TestFollowingWithNothingConnected(t *testing.T) {
	s := &States{}

	v := s.Follow("sensor.hallway", "")
	if v.Known() {
		t.Error("a value nobody has reported is known")
	}
}

func TestStatesArriveAtTheValueTheyAreFor(t *testing.T) {
	s := &States{}
	state := s.Follow("weather.home", "")
	temp := s.Follow("weather.home", "temperature")

	report(t, s, "weather.home", "", "cloudy")
	report(t, s, "weather.home", "temperature", "18.4")

	if got, _ := state.Get(); got != "cloudy" {
		t.Errorf("state = %q, want cloudy", got)
	}
	if got, ok := temp.Float(); !ok || got != 18.4 {
		t.Errorf("temperature = %v (%v), want 18.4", got, ok)
	}
}

// A report for something nothing follows is dropped rather than kept: the list is what the device
// asked for, and anything else is the server's business.
func TestAStateNothingFollowsIsDropped(t *testing.T) {
	s := &States{}
	v := s.Follow("weather.home", "")

	report(t, s, "sensor.elsewhere", "", "12")

	if v.Known() {
		t.Error("a report for another entity landed on this one")
	}
}

// Home Assistant reports the current value on every connection, and a redraw for a value that has
// not moved is a redraw for nothing.
func TestChangedOnlyFiresOnNews(t *testing.T) {
	s := &States{}
	v := s.Follow("weather.home", "")

	var fired []string
	v.Changed.Listen(func(s string) { fired = append(fired, s) })

	report(t, s, "weather.home", "", "cloudy")
	report(t, s, "weather.home", "", "cloudy")
	report(t, s, "weather.home", "", "sunny")

	want := []string{"cloudy", "sunny"}
	if len(fired) != len(want) {
		t.Fatalf("fired %v, want %v", fired, want)
	}
	for i := range want {
		if fired[i] != want[i] {
			t.Errorf("change %d is %q, want %q", i, fired[i], want[i])
		}
	}
}

// Everything arrives as a string, including the states that are plainly numbers, and an entity
// with nothing behind it sends a word.
func TestOnlyANumberReadsAsANumber(t *testing.T) {
	tests := []struct {
		state string
		want  float64
		ok    bool
	}{
		{"18.4", 18.4, true},
		{" 21 ", 21, true},
		{"-3.5", -3.5, true},
		{"unavailable", 0, false},
		{"unknown", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		s := &States{}
		v := s.Follow("sensor.thing", "")
		report(t, s, "sensor.thing", "", tt.state)

		got, ok := v.Float()
		if ok != tt.ok || got != tt.want {
			t.Errorf("Float(%q) = %v, %v; want %v, %v", tt.state, got, ok, tt.want, tt.ok)
		}
	}
}

// An entity nobody named is not followed, rather than followed under an empty name that every
// report with no entity would match.
func TestAnEmptyEntityIsNotFollowed(t *testing.T) {
	s := &States{}

	s.Follow("", "")
	s.Follow("   ", "temperature")

	if got := len(s.Following()); got != 0 {
		t.Errorf("%d subscriptions, want none", got)
	}
}

// The handler is what the server calls, and the two messages it cares about have to go to the
// right halves.
func TestTheHandlerAnswersTheRequestAndTakesTheReports(t *testing.T) {
	s := &States{}
	v := s.Follow("weather.home", "")

	ctx := context.Background()

	// No connection to answer on, which is the shape a test can build: what matters is that
	// neither message is refused.
	if err := s.Handle(ctx, nil, &api.SubscribeHomeAssistantStatesRequest{}); err != nil {
		t.Fatalf("the request was refused: %v", err)
	}
	if err := s.Handle(ctx, nil, &api.HomeAssistantStateResponse{
		EntityId: "weather.home", State: "rainy",
	}); err != nil {
		t.Fatalf("the report was refused: %v", err)
	}

	if got, _ := v.Get(); got != "rainy" {
		t.Errorf("state = %q, want rainy", got)
	}
}

// Anything else on the connection is not this package's, and passing it through has to be free.
func TestTheHandlerIgnoresEverythingElse(t *testing.T) {
	s := &States{}

	if err := s.Handle(context.Background(), nil, &api.PingRequest{}); err != nil {
		t.Errorf("an unrelated message gave %v", err)
	}
}

func report(t *testing.T, s *States, entity, attribute, state string) {
	t.Helper()
	s.take(entity, attribute, state)
}
