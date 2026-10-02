package cast

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTheSetupEndpointServesTheDescription(t *testing.T) {
	e := NewEureka(Device{ID: "266a1baa915744d5e2531b227c645271", Name: "dev", Model: "LANovo"}, "Lenovo", "1")
	var seen []int
	h := SetupHandler(func() Eureka { return e }, func(_ *http.Request, status int) { seen = append(seen, status) })

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/setup/eureka_info?params=name,device_info.ssdp_udn", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["name"] != "dev" {
		t.Errorf("name %v", got["name"])
	}
	info, _ := got["device_info"].(map[string]any)
	if info["ssdp_udn"] != "266a1baa-9157-44d5-e253-1b227c645271" {
		t.Errorf("device_info %v", got["device_info"])
	}
	if _, ok := got["version"]; ok {
		t.Errorf("unasked field in %v", got)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/setup/eureka_info", nil))
	var flat map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &flat); err != nil {
		t.Fatal(err)
	}
	if flat["name"] != "dev" || flat["device_info"] != nil {
		t.Errorf("no params gave %v", flat)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/setup/other", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("other path status %d", rec.Code)
	}
	if len(seen) != 3 || seen[2] != http.StatusNotFound {
		t.Errorf("seen %v", seen)
	}
}
