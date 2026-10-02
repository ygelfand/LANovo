package cast

import (
	"encoding/json"
	"testing"
)

var kitchen = Device{ID: "1b0cf94a40f35185c64b074179c1caa8", Name: "Kitchen", Model: "LANovo"}

func answer(t *testing.T, r *Receiver, request string) (envelope map[string]any, data map[string]any) {
	t.Helper()
	out, err := r.Receive(from(SenderID, NSSetup, request))
	if err != nil || len(out) != 1 {
		t.Fatalf("%d answers, %v", len(out), err)
	}
	if out[0].Namespace != NSSetup || out[0].Destination != SenderID {
		t.Errorf("answered on %s to %s", out[0].Namespace, out[0].Destination)
	}
	if err := json.Unmarshal([]byte(out[0].Payload), &envelope); err != nil {
		t.Fatal(err)
	}
	data, _ = envelope["data"].(map[string]any)
	return envelope, data
}

func eurekaReceiver() *Receiver {
	r := NewReceiver("Kitchen")
	r.Eureka = func() Eureka { return NewEureka(kitchen, "LANovo", "0.0.1") }
	return r
}

func TestEurekaInfoWithNoParamsIsTheFlatSet(t *testing.T) {
	env, data := answer(t, eurekaReceiver(), `{"type":"eureka_info","request_id":7,"data":{}}`)

	if env["type"] != TypeEurekaInfo || env["request_id"] != float64(7) ||
		env["response_code"] != float64(200) || env["response_string"] != "OK" {
		t.Errorf("the envelope is %v", env)
	}
	if data["name"] != "Kitchen" || data["version"] != float64(EurekaVersion) ||
		data["ssdp_udn"] != "1b0cf94a-40f3-5185-c64b-074179c1caa8" || data["setup_state"] != float64(SetupDone) {
		t.Errorf("the data is %v", data)
	}
	for _, g := range groups {
		if _, ok := data[g]; ok {
			t.Errorf("%s came back without being asked for", g)
		}
	}
}

func TestEurekaInfoWithParamsIsOnlyThosePaths(t *testing.T) {
	_, data := answer(t, eurekaReceiver(), `{"type":"eureka_info","request_id":8,"data":{"params":[`+
		`"version","name","multizone","device_info.ssdp_udn","device_info.manufacturer",`+
		`"device_info.product_name","build_info.build_type","build_info.cast_build_revision",`+
		`"build_info.system_build_number","device_info.capabilities.multiplexed_connections_supported",`+
		`"device_info.cloud_device_id"]}}`)

	want := map[string]bool{"version": true, "name": true, "multizone": true, "device_info": true, "build_info": true}
	for k := range data {
		if !want[k] {
			t.Errorf("%s came back without being asked for", k)
		}
	}
	for k := range want {
		if _, ok := data[k]; !ok {
			t.Errorf("%s was asked for and is missing", k)
		}
	}

	info := data["device_info"].(map[string]any)
	if info["cloud_device_id"] != kitchen.CloudID() || info["product_name"] != "LANovo" ||
		info["ssdp_udn"] != "1b0cf94a-40f3-5185-c64b-074179c1caa8" {
		t.Errorf("device_info is %v", info)
	}
	if _, ok := info["capabilities"]; ok {
		t.Error("a path the device does not have came back")
	}
	build := data["build_info"].(map[string]any)
	if build["build_type"] != float64(BuildType) || build["cast_build_revision"] != CastBuild {
		t.Errorf("build_info is %v", build)
	}
	if groups := data["multizone"].(map[string]any)["groups"]; groups == nil {
		t.Error("multizone has no groups list")
	}
}

func TestParamsMayBeACommaSeparatedString(t *testing.T) {
	_, data := answer(t, eurekaReceiver(), `{"type":"eureka_info","request_id":9,"data":{"params":"name,build_info.build_type"}}`)
	if data["name"] != "Kitchen" || len(data) != 2 {
		t.Errorf("the data is %v", data)
	}
}

func TestTheCloudIDIsTheOneAdvertised(t *testing.T) {
	for _, rec := range kitchen.Records() {
		if rec == "cd="+kitchen.CloudID() {
			return
		}
	}
	t.Errorf("no cd=%s in %v", kitchen.CloudID(), kitchen.Records())
}

func TestOtherSetupRequestsAreReportedAndUnanswered(t *testing.T) {
	r := NewReceiver("Kitchen")
	var unspoken [][2]string
	r.Unspoken = func(namespace, kind string) { unspoken = append(unspoken, [2]string{namespace, kind}) }

	out, err := r.Receive(from(SenderID, NSSetup, `{"type":"set_eureka_info","request_id":8,"data":{"name":"x"}}`))
	if err != nil || len(out) != 0 {
		t.Fatalf("%d answers, %v", len(out), err)
	}
	if len(unspoken) != 1 || unspoken[0] != [2]string{NSSetup, "set_eureka_info"} {
		t.Errorf("reported %v", unspoken)
	}
}

func TestTheUDNIsTheIDDashed(t *testing.T) {
	if got := kitchen.UDN(); got != "1b0cf94a-40f3-5185-c64b-074179c1caa8" {
		t.Errorf("the udn is %s", got)
	}
}
