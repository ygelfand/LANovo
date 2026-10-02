package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ygelfand/go-esphome-device/api"
	"google.golang.org/protobuf/proto"
)

type fake struct {
	h      *HomeAssistant
	mu     sync.Mutex
	answer func(*api.HomeassistantActionRequest) *api.HomeassistantActionResponse
	fail   error
}

func (f *fake) Send(m proto.Message) error {
	if f.fail != nil {
		return f.fail
	}
	req := m.(*api.HomeassistantActionRequest)
	f.mu.Lock()
	answer := f.answer
	f.mu.Unlock()
	if answer == nil {
		return nil
	}
	if resp := answer(req); resp != nil {
		resp.CallId = req.GetCallId()
		go func() { _ = f.h.Handle(context.Background(), nil, resp) }()
	}
	return nil
}

func answering(data string) func(*api.HomeassistantActionRequest) *api.HomeassistantActionResponse {
	return func(*api.HomeassistantActionRequest) *api.HomeassistantActionResponse {
		return &api.HomeassistantActionResponse{Success: true, ResponseData: []byte(data)}
	}
}

func connected(answer func(*api.HomeassistantActionRequest) *api.HomeassistantActionResponse) (*HomeAssistant, *fake) {
	h := &HomeAssistant{waiting: map[uint32]chan reply{}}
	f := &fake{h: h, answer: answer}
	h.to = f
	return h, f
}

func waitFor(t *testing.T, h *HomeAssistant, want Access) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for h.Access() != want {
		if time.Now().After(deadline) {
			t.Fatalf("access is %s, want %s", h.Access(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestProbeAllows(t *testing.T) {
	h, f := connected(answering(`{"response":"ok"}`))
	var heard []Access
	var mu sync.Mutex
	h.Changed.Listen(func(a Access) { mu.Lock(); heard = append(heard, a); mu.Unlock() })

	h.reprobe(f)
	waitFor(t, h, Allowed)

	mu.Lock()
	defer mu.Unlock()
	if len(heard) != 2 || heard[0] != Unknown || heard[1] != Allowed {
		t.Errorf("heard %v, want [unknown allowed]", heard)
	}
}

func TestRefusedFailsFastAndReprobes(t *testing.T) {
	h, f := connected(answering(`{"response":"ok"}`))
	h.access = Refused

	if err := h.Call(context.Background(), "light.turn_on", nil); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("a call while refused said %v, want ErrNotAllowed", err)
	}

	h.Probe()
	waitFor(t, h, Allowed)
	_ = f
}

func TestSendFailureDisconnects(t *testing.T) {
	h, f := connected(nil)
	f.fail = errors.New("closed")
	var heard Access = Allowed
	h.Changed.Listen(func(a Access) { heard = a })

	if err := h.Call(context.Background(), "light.turn_on", nil); err == nil {
		t.Fatal("a failed send reported success")
	}
	if h.Connected() {
		t.Error("still connected after the send failed")
	}
	if heard != Unknown {
		t.Errorf("heard %s, want unknown", heard)
	}
	if err := h.Call(context.Background(), "light.turn_on", nil); !errors.Is(err, ErrNotConnected) {
		t.Errorf("the next call said %v, want ErrNotConnected", err)
	}
}

func TestEntitiesSendsTheFilter(t *testing.T) {
	var sent string
	h, _ := connected(func(req *api.HomeassistantActionRequest) *api.HomeassistantActionResponse {
		sent = req.GetResponseTemplate()
		return &api.HomeassistantActionResponse{Success: true, ResponseData: []byte(`{"response":"[{\"id\":\"light.a\",\"name\":\"A\",\"state\":\"on\",\"area\":\"\"}]"}`)}
	})
	h.access = Allowed

	got, err := h.Entities(context.Background(), Filter{Domains: []string{"light"}, Labels: []string{"lanovo"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "light.a" {
		t.Errorf("got %+v", got)
	}
	for _, want := range []string{`selectattr('domain', 'in', ["light"])`, `select('in', ["lanovo"])`, `in []`} {
		if !strings.Contains(sent, want) {
			t.Errorf("the template lacks %q:\n%s", want, sent)
		}
	}
}

func TestEveryDomainWithoutAFilter(t *testing.T) {
	tmpl := Filter{}.template()
	if strings.Contains(tmpl, "selectattr") || !strings.Contains(tmpl, "for s in states -%}") {
		t.Errorf("an empty filter should walk every state:\n%s", tmpl)
	}
}

func TestParseEntitiesSortsByAreaThenName(t *testing.T) {
	list := `[{"id":"light.z","name":"zed","area":""},{"id":"light.b","name":"Bee","area":"kitchen"},{"id":"light.a","name":"Ay","area":"Kitchen"},{"id":"light.c","name":"Cee","area":"Attic"}]`
	quoted, _ := json.Marshal(list)
	for _, raw := range []string{list, string(quoted)} {
		got, err := parseEntities(json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, e := range got {
			ids = append(ids, e.ID)
		}
		if strings.Join(ids, " ") != "light.c light.a light.b light.z" {
			t.Errorf("order %v", ids)
		}
	}
}

func TestParseEmptyIsNotNil(t *testing.T) {
	got, err := parseEntities(json.RawMessage(`"[]"`))
	if err != nil || got == nil {
		t.Errorf("got %v, %v; want an empty list", got, err)
	}
}
