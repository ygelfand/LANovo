package cast

import (
	"encoding/json"
	"testing"
)

func TestDeviceInfoIsAnsweredTheWayAStockDeviceAnswers(t *testing.T) {
	r := NewReceiver("Kitchen")
	r.Identity = func() Device { return kitchen }

	out, err := r.Receive(from(SenderID, NSDiscovery, `{"type":"GET_DEVICE_INFO","requestId":5}`))
	if err != nil || len(out) != 1 {
		t.Fatalf("%d answers, %v", len(out), err)
	}
	if out[0].Namespace != NSDiscovery || out[0].Destination != SenderID {
		t.Errorf("answered on %s to %s", out[0].Namespace, out[0].Destination)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out[0].Payload), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"type": TypeDeviceInfo, "requestId": float64(5), "deviceId": kitchen.ID,
		"friendlyName": "Kitchen", "deviceModel": "LANovo",
		"deviceCapabilities": float64(InfoCapabilities), "deviceIconUrl": "/setup/icon.png",
		"controlNotifications": float64(1), "wifiProximityId": kitchen.Proximity(),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s is %v, want %v", k, got[k], v)
		}
	}
}

func TestTheProximityIDIsTheOneAdvertised(t *testing.T) {
	for _, rec := range kitchen.Records() {
		if rec == "bs="+kitchen.Proximity() {
			return
		}
	}
	t.Errorf("no bs=%s in %v", kitchen.Proximity(), kitchen.Records())
}
