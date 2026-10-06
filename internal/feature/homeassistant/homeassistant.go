package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"
	"google.golang.org/protobuf/proto"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

func init() {
	component.Register(component.Device, Get, component.Order(5))
}

var (
	ErrNotConnected = errors.New("homeassistant: not connected")
	ErrNotAllowed   = errors.New("homeassistant: actions are not allowed for this device; enable them in the ESPHome integration's options")
)

const (
	probeWait = 5 * time.Second
	syncWait  = 30 * time.Second
)

type Access int

const (
	Unknown Access = iota
	Allowed
	Refused
)

func (a Access) String() string {
	switch a {
	case Allowed:
		return "allowed"
	case Refused:
		return "refused"
	}
	return "unknown"
}

type sender interface {
	Send(proto.Message) error
}

type reply struct {
	data []byte
	err  error
}

type HomeAssistant struct {
	Changed hook.Hook[Access]

	mu      sync.Mutex
	to      sender
	access  Access
	next    uint32
	waiting map[uint32]chan reply
	born    time.Time
}

var (
	once   sync.Once
	shared *HomeAssistant
)

func Get() *HomeAssistant {
	once.Do(func() { shared = &HomeAssistant{waiting: map[uint32]chan reply{}, born: time.Now()} })
	return shared
}

func (h *HomeAssistant) Name() string { return "homeassistant" }

func (h *HomeAssistant) Handle(_ context.Context, conn *esphome.Conn, msg proto.Message) error {
	switch m := msg.(type) {
	case *api.SubscribeHomeassistantServicesRequest:
		h.mu.Lock()
		h.to = conn
		h.mu.Unlock()
		h.reprobe(conn)
	case *api.HomeassistantActionResponse:
		h.mu.Lock()
		ch := h.waiting[m.GetCallId()]
		delete(h.waiting, m.GetCallId())
		h.mu.Unlock()
		if ch == nil {
			return nil
		}
		if !m.GetSuccess() {
			ch <- reply{err: fmt.Errorf("homeassistant: %s", m.GetErrorMessage())}
			return nil
		}
		ch <- reply{data: m.GetResponseData()}
	}
	return nil
}

func (h *HomeAssistant) Startup() component.Progress {
	return h.startup(config.Get().API.Adopted, time.Now())
}

func (h *HomeAssistant) startup(adopted bool, now time.Time) component.Progress {
	if !adopted {
		return component.Progress{Done: true, Doing: "not adopted"}
	}
	if !config.Get().Home.Enabled {
		return component.Progress{Done: true, Doing: "home control off"}
	}
	switch h.Access() {
	case Allowed:
		return component.Progress{Done: true, Doing: "synced"}
	case Refused:
		return component.Progress{Failed: true, Doing: "actions not allowed"}
	}
	if now.Sub(h.born) > syncWait {
		return component.Progress{Failed: true, Doing: "Home Assistant has not connected"}
	}
	if h.Connected() {
		return component.Progress{Doing: "checking access"}
	}
	return component.Progress{Doing: "waiting for Home Assistant"}
}

func (h *HomeAssistant) Access() Access {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.access
}

func (h *HomeAssistant) Connected() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.to != nil
}

func (h *HomeAssistant) Probe() {
	h.mu.Lock()
	conn := h.to
	h.mu.Unlock()
	if conn != nil {
		h.reprobe(conn)
	}
}

func (h *HomeAssistant) reprobe(conn sender) {
	if !config.Get().Home.Enabled {
		return
	}
	h.mu.Lock()
	current := h.to == conn
	if current {
		h.access = Unknown
	}
	h.mu.Unlock()
	if !current {
		return
	}
	h.Changed.Emit(Unknown)
	go h.probe(conn)
}

func (h *HomeAssistant) probe(conn sender) {
	ctx, cancel := context.WithTimeout(context.Background(), probeWait)
	defer cancel()
	_, err := h.call(ctx, carrier, carrierData(), "ok")
	switch {
	case err == nil:
		h.settle(conn, Allowed)
	case errors.Is(err, context.DeadlineExceeded):
		slog.Warn("Home Assistant does not answer actions from this device; enable them in the ESPHome integration's options")
		h.settle(conn, Refused)
	}
}

func (h *HomeAssistant) Enable(on bool) {
	if on {
		h.Probe()
		return
	}
	h.mu.Lock()
	moved := h.access != Unknown
	h.access = Unknown
	h.mu.Unlock()
	if moved {
		h.Changed.Emit(Unknown)
	}
}

func (h *HomeAssistant) settle(conn sender, access Access) {
	h.mu.Lock()
	moved := h.to == conn && h.access != access
	if moved {
		h.access = access
	}
	h.mu.Unlock()
	if moved {
		h.Changed.Emit(access)
	}
}

func (h *HomeAssistant) lost(conn sender) {
	h.mu.Lock()
	gone := h.to == conn
	if gone {
		h.to = nil
		h.access = Unknown
	}
	h.mu.Unlock()
	if gone {
		h.Changed.Emit(Unknown)
	}
}

func (h *HomeAssistant) Call(ctx context.Context, action string, data map[string]string) error {
	_, err := h.call(ctx, action, data, "")
	return err
}

func (h *HomeAssistant) Template(ctx context.Context, tmpl string) (json.RawMessage, error) {
	raw, err := h.call(ctx, carrier, carrierData(), tmpl)
	if err != nil {
		return nil, err
	}
	var out struct {
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("homeassistant: the response: %w", err)
	}
	return out.Response, nil
}

const carrier = "recorder.get_statistics"

func carrierData() map[string]string {
	return map[string]string{
		"statistic_ids": "lanovo.none",
		"start_time":    time.Now().UTC().Format(time.RFC3339),
		"period":        "hour",
		"types":         "state",
	}
}

func (h *HomeAssistant) call(ctx context.Context, action string, data map[string]string, tmpl string) ([]byte, error) {
	ch := make(chan reply, 1)
	h.mu.Lock()
	to, access := h.to, h.access
	h.next++
	id := h.next
	if to != nil {
		h.waiting[id] = ch
	}
	h.mu.Unlock()
	if to == nil {
		return nil, ErrNotConnected
	}
	if access == Refused {
		h.mu.Lock()
		delete(h.waiting, id)
		h.mu.Unlock()
		return nil, ErrNotAllowed
	}
	defer func() {
		h.mu.Lock()
		delete(h.waiting, id)
		h.mu.Unlock()
	}()

	fields := make([]*api.HomeassistantServiceMap, 0, len(data))
	for k, v := range data {
		fields = append(fields, &api.HomeassistantServiceMap{Key: k, Value: v})
	}
	req := &api.HomeassistantActionRequest{
		Service:          action,
		Data:             fields,
		CallId:           id,
		WantsResponse:    tmpl != "",
		ResponseTemplate: tmpl,
	}
	if err := to.Send(req); err != nil {
		h.lost(to)
		return nil, err
	}

	select {
	case r := <-ch:
		return r.data, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
