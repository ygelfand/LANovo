package cast

import (
	"encoding/json"
	"testing"
)

func availabilityOf(t *testing.T, r *Receiver) map[string]any {
	t.Helper()
	out, err := r.Receive(from(SenderID, NSReceiver,
		`{"type":"GET_APP_AVAILABILITY","appId":["2DB7CC49","CFE7FEDA"],"requestId":1}`))
	if err != nil || len(out) != 1 {
		t.Fatalf("%d answers, %v", len(out), err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out[0].Payload), &got); err != nil {
		t.Fatal(err)
	}
	if got["responseType"] != TypeGetAppAvailability || got["requestId"] != float64(1) {
		t.Errorf("the envelope is %v", got)
	}
	a, _ := got["availability"].(map[string]any)
	return a
}

func TestEveryApplicationIsAvailableByDefault(t *testing.T) {
	a := availabilityOf(t, NewReceiver("Kitchen"))
	if a["2DB7CC49"] != AppAvailable || a["CFE7FEDA"] != AppAvailable {
		t.Errorf("availability is %v", a)
	}
}

func TestAvailabilityCanBeNarrowed(t *testing.T) {
	r := NewReceiver("Kitchen")
	r.Available = func(app string) bool { return app == "2DB7CC49" }

	a := availabilityOf(t, r)
	if a["2DB7CC49"] != AppAvailable || a["CFE7FEDA"] != AppUnavailable {
		t.Errorf("availability is %v", a)
	}
}
