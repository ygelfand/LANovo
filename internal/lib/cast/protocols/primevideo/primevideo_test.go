package primevideo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/cast"
)

type memory struct{ data []byte }

func (m *memory) Load(v any) bool { return m.data != nil && json.Unmarshal(m.data, v) == nil }
func (m *memory) Save(v any) error {
	b, err := json.Marshal(v)
	m.data = b
	return err
}
func (m *memory) Clear() error { m.data = nil; return nil }

func message(body string) cast.Message {
	return cast.Message{Source: "sender-1", Destination: "transport-1", Namespace: NS, Payload: body}
}

func answered(t *testing.T, out []cast.Message) answer {
	t.Helper()
	if len(out) != 1 || out[0].Destination != "sender-1" || out[0].Source != "transport-1" || out[0].Namespace != NS {
		t.Fatalf("out %+v", out)
	}
	var a answer
	if err := json.Unmarshal([]byte(out[0].Payload), &a); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRegistration(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	amazon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/auth/register":
			w.Write([]byte(`{"response":{"success":{"tokens":{"bearer":{"refresh_token":"rt"}}}}}`))
		case "/auth/token":
			w.Write([]byte(`{"device_tokens":[{"actor_access_token":{"token":"at"}}]}`))
		}
	}))
	defer amazon.Close()

	sent := make(chan cast.Message, 1)
	store := &memory{}
	p := &Protocol{
		env:       cast.Env{HTTP: amazon.Client(), Send: func(m cast.Message) { sent <- m }},
		auth:      amazon.URL,
		kept:      store,
		persist:   func() bool { return true },
		envelopes: map[string]Envelope{},
	}

	out, err := p.Receive(nil, message(`{"deviceId":"abcGCAST","messageProtocolVersion":1,"type":"AmIRegistered"}`))
	if err != nil {
		t.Fatal(err)
	}
	if a := answered(t, out); a.Type != "AmIRegisteredResponse" || a.Error == nil || a.Error.Code != "NotRegistered" || !strings.Contains(a.Error.Message, "abcGCAST") {
		t.Fatalf("unregistered %+v", a)
	}

	out, err = p.Receive(nil, message(`{"type":"Register","deviceId":"abcGCAST","actorId":"actor","marketplaceId":"ATVPDKIKX0DER","preAuthorizedLinkCode":"code"}`))
	if err != nil || len(out) != 0 {
		t.Fatalf("register answered at once %+v %v", out, err)
	}
	select {
	case m := <-sent:
		if a := answered(t, []cast.Message{m}); a.Type != "RegisterResponse" || a.Error != nil {
			t.Fatalf("register %+v", a)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no register answer")
	}
	var saved Registration
	store.Load(&saved)
	if saved.Refresh != "rt" || saved.Actor != "actor" || saved.Marketplace != "ATVPDKIKX0DER" || saved.Device != "abcGCAST" || p.access != "at" {
		t.Fatalf("saved %+v access %q", saved, p.access)
	}
	if strings.Join(paths, ",") != "/auth/register,/auth/token" {
		t.Fatalf("paths %v", paths)
	}

	out, _ = p.Receive(nil, message(`{"type":"AmIRegistered","deviceId":"abcGCAST"}`))
	if a := answered(t, out); a.Error != nil {
		t.Fatalf("registered %+v", a)
	}

	out, _ = p.Receive(nil, message(`{"type":"ApplySettings","settings":{"locale":"en-US"}}`))
	if a := answered(t, out); a.Type != "ApplySettingsResponse" || p.locale != "en_US" {
		t.Fatalf("settings %+v %q", a, p.locale)
	}

	out, _ = p.Receive(nil, message(`{"type":"Preload","contentId":"amzn1.dv.gti.x","playbackEnvelope":{"envelope":"env","correlationId":"c"}}`))
	if a := answered(t, out); a.Type != "PreloadResponse" || p.envelopes["amzn1.dv.gti.x"].Envelope != "env" {
		t.Fatalf("preload %+v", a)
	}
}

func TestRegistrationLastsOneSessionUnlessKept(t *testing.T) {
	reg := Registration{Actor: "actor", Refresh: "rt", Device: "abcGCAST"}
	store := &memory{}
	store.Save(reg)
	keep := false
	p := &Protocol{
		kept:      store,
		persist:   func() bool { return keep },
		saved:     reg,
		envelopes: map[string]Envelope{},
	}
	registered := func() bool {
		out, _ := p.Receive(nil, message(`{"type":"AmIRegistered","deviceId":"abcGCAST"}`))
		return answered(t, out).Error == nil
	}
	if !registered() {
		t.Fatal("not registered within the session")
	}
	p.Started(cast.App{})
	if registered() {
		t.Fatal("a new session inherited the last caster's registration")
	}
	keep = true
	p.Started(cast.App{})
	if !registered() {
		t.Fatal("a kept registration was dropped")
	}
	p.ForgetRegistration()
	if registered() || store.data != nil {
		t.Fatal("reset left a registration behind")
	}
}
