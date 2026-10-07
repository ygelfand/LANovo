package wifi

import "testing"

func TestConnectedEventCompletesStartupWithoutVendorStateEvent(t *testing.T) {
	r := &Radio{ssid: "home"}
	r.connected()
	if p := r.Startup(); !p.Done || r.Network() != "home" {
		t.Fatalf("CONNECTED did not update association: %+v", p)
	}
	r.apply(false, "")
	if p := r.Startup(); p.Done || r.Network() != "" {
		t.Fatalf("disconnect retained association: %+v", p)
	}
}
