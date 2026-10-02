package youtube

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/ygelfand/LANovo/internal/lib/cast"
)

const music = "2DB7CC49"

type screensFunc = func(app string) (string, string, bool)

func launched(t *testing.T, screens screensFunc) (*cast.Receiver, string) {
	t.Helper()
	r := cast.NewReceiver("Kitchen")
	r.Register(&Protocol{screens: screens})

	out, err := r.Receive(cast.Message{Source: cast.SenderID, Destination: cast.ReceiverID,
		Namespace: cast.NSReceiver, Payload: `{"type":"LAUNCH","requestId":1,"appId":"` + music + `"}`})
	if err != nil || len(out) == 0 {
		t.Fatalf("launch: %d answers, %v", len(out), err)
	}
	var st struct {
		Status cast.ReceiverStatus `json:"status"`
	}
	json.Unmarshal([]byte(out[0].Payload), &st)
	app := st.Status.Applications[0]

	var names []string
	for _, n := range app.Namespaces {
		names = append(names, n.Name)
	}
	if want := []string{cast.NSMedia, NSMDX, NSCAC, NSDebugOverlay}; !slices.Equal(names, want) {
		t.Errorf("the running app speaks %v, want %v", names, want)
	}
	return r, app.TransportID
}

func TestGetMdxSessionStatusIsAnsweredWithTheScreen(t *testing.T) {
	r, transport := launched(t, func(app string) (string, string, bool) {
		if app != music {
			t.Errorf("asked for the screen of %s", app)
		}
		return "75isi1kidcmhncekdrf198ol0b", "1290280e-38d3-4fd3-96c1-d1f25e7258ed", true
	})

	out, err := r.Receive(cast.Message{Source: "phone-1", Destination: transport,
		Namespace: NSMDX, Payload: `{"type":"getMdxSessionStatus"}`})
	if err != nil || len(out) != 1 {
		t.Fatalf("%d answers, %v", len(out), err)
	}
	if out[0].Namespace != NSMDX || out[0].Destination != "phone-1" || out[0].Source != transport {
		t.Errorf("answered %+v", out[0])
	}
	want := `{"type":"mdxSessionStatus","data":{"screenId":"75isi1kidcmhncekdrf198ol0b","deviceId":"1290280e-38d3-4fd3-96c1-d1f25e7258ed"}}`
	if out[0].Payload != want {
		t.Errorf("answered %s, want %s", out[0].Payload, want)
	}
}

func TestWithNoScreenYetNothingIsAnswered(t *testing.T) {
	r, transport := launched(t, func(string) (string, string, bool) { return "", "", false })

	var unspoken []string
	r.Unspoken = func(namespace, kind string) { unspoken = append(unspoken, kind) }

	out, err := r.Receive(cast.Message{Source: "phone-1", Destination: transport,
		Namespace: NSMDX, Payload: `{"type":"getMdxSessionStatus"}`})
	if err != nil || len(out) != 0 {
		t.Fatalf("%d answers, %v", len(out), err)
	}
	if len(unspoken) != 1 {
		t.Errorf("reported %v", unspoken)
	}
}

func TestAnAppThatDoesNotNameTheProtocolDoesNotSpeakIt(t *testing.T) {
	r := cast.NewReceiver("Kitchen")
	r.Register(&Protocol{screens: func(string) (string, string, bool) { return "s", "d", true }})
	r.Receive(cast.Message{Source: cast.SenderID, Destination: cast.ReceiverID,
		Namespace: cast.NSReceiver, Payload: `{"type":"LAUNCH","requestId":1,"appId":"C35B0678"}`})

	out, _ := r.Receive(cast.Message{Source: "phone-1", Destination: "x",
		Namespace: NSMDX, Payload: `{"type":"getMdxSessionStatus"}`})
	if len(out) != 0 {
		t.Errorf("an app without the protocol answered %v", out)
	}
}
