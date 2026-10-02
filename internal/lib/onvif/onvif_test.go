package onvif

import (
	"encoding/xml"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const probe = `<?xml version="1.0" encoding="utf-8"?>
<Envelope xmlns:tds="http://www.onvif.org/ver10/device/wsdl" xmlns="http://www.w3.org/2003/05/soap-envelope">
<Header><wsa:MessageID xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing">uuid:9d1b8a6e-0000-4000-8000-000000000001</wsa:MessageID>
<wsa:To xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing">urn:schemas-xmlsoap-org:ws:2005:04:discovery</wsa:To>
<wsa:Action xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing">http://schemas.xmlsoap.org/ws/2005/04/discovery/Probe</wsa:Action></Header>
<Body><Probe xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns="http://schemas.xmlsoap.org/ws/2005/04/discovery">
<Types>tds:Device</Types><Scopes /></Probe></Body></Envelope>`

func TestAProbeIsAnsweredWithTheDevice(t *testing.T) {
	p, ok := ParseProbe([]byte(probe))
	if !ok || !p.Wants() {
		t.Fatalf("probe read as %+v ok=%v", p, ok)
	}
	reply := Matches(p, "1234-abcd", "http://10.0.0.5:8000/onvif/device_service", "Kitchen display")
	var env struct {
		Header struct {
			RelatesTo string `xml:"RelatesTo"`
		} `xml:"Header"`
		Body struct {
			Matches struct {
				Match struct {
					Types  string `xml:"Types"`
					XAddrs string `xml:"XAddrs"`
					Scopes string `xml:"Scopes"`
				} `xml:"ProbeMatch"`
			} `xml:"ProbeMatches"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(reply, &env); err != nil {
		t.Fatalf("the reply is not XML: %v\n%s", err, reply)
	}
	m := env.Body.Matches.Match
	if env.Header.RelatesTo != p.MessageID {
		t.Errorf("RelatesTo %q, want %q", env.Header.RelatesTo, p.MessageID)
	}
	if !strings.Contains(m.Types, "NetworkVideoTransmitter") || m.XAddrs != "http://10.0.0.5:8000/onvif/device_service" {
		t.Errorf("match %+v", m)
	}
	if !strings.Contains(m.Scopes, "name/Kitchen%20display") {
		t.Errorf("scopes %q", m.Scopes)
	}
	if _, ok := ParseProbe(reply); ok {
		t.Error("our own ProbeMatches read as a probe")
	}
}

func TestProbeTypes(t *testing.T) {
	for types, want := range map[string]bool{
		"": true, "dn:NetworkVideoTransmitter": true, "tds:Device": true, "wsdp:Device": true, "d:Printer": false,
	} {
		if got := (Probe{Types: types}).Wants(); got != want {
			t.Errorf("%q wants=%v", types, got)
		}
	}
}

func soap(t *testing.T, url, op, inner string) (int, string) {
	t.Helper()
	body := `<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:trt="http://www.onvif.org/ver10/media/wsdl" xmlns:tds="http://www.onvif.org/ver10/device/wsdl"><s:Body><` + op + `>` + inner + `</` + op + `></s:Body></s:Envelope>`
	r, err := http.Post(url, "application/soap+xml", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	var any struct{}
	if err := xml.Unmarshal(b, &any); err != nil {
		t.Fatalf("%s answered something that is not XML: %v\n%s", op, err, b)
	}
	return r.StatusCode, string(b)
}

func TestServeForAClient(t *testing.T) {
	hold, err := time.ParseDuration(os.Getenv("LANOVO_ONVIF_SERVE"))
	if err != nil {
		t.Skip("set LANOVO_ONVIF_SERVE to a duration to serve on 127.0.0.1:8000")
	}
	s := &Service{Device: Device{Manufacturer: "Lenovo", Model: "SD-X701B", Firmware: "1.0", Serial: "HUA03RW0", Name: "display", MAC: "aa:bb:cc:dd:ee:ff", RTSPPort: 8554,
		Profiles: func() []Profile {
			return []Profile{{Token: "main", Name: "Main", Width: 2592, Height: 1944, FPS: 30, Bitrate: 6_000_000, Path: "main"},
				{Token: "sub", Name: "Sub", Width: 1280, Height: 720, FPS: 10, Bitrate: 921_600, Path: "sub"}}
		}}}
	mux := http.NewServeMux()
	mux.Handle(DevicePath, s)
	mux.Handle(MediaPath, s)
	srv := &http.Server{Addr: "127.0.0.1:8000", Handler: mux}
	go srv.ListenAndServe()
	r := &Responder{UUID: NewUUID(), Name: "display", XAddr: func(ip net.IP) string {
		return "http://" + ip.String() + ":8000" + DevicePath
	}}
	if err := r.Listen(); err != nil {
		t.Logf("discovery not answering: %v", err)
	} else {
		defer r.Close()
	}
	time.Sleep(hold)
	srv.Close()
}

func TestTheServicesDescribeTheCamera(t *testing.T) {
	s := &Service{
		Device: Device{Manufacturer: "Lenovo", Model: "SD-X701B", Firmware: "1.0", Serial: "HUA03RW0", Name: "display", MAC: "aa:bb:cc:dd:ee:ff", RTSPPort: 8554,
			Profiles: func() []Profile {
				return []Profile{{Token: "main", Name: "Main", Width: 2592, Height: 1944, FPS: 30, Bitrate: 6_000_000, Path: "main"},
					{Token: "sub", Name: "Sub", Width: 1280, Height: 720, FPS: 10, Bitrate: 921_600, Path: "sub"}}
			}},
		Now: func() time.Time { return time.Date(2026, 9, 27, 13, 4, 5, 0, time.UTC) },
	}
	mux := http.NewServeMux()
	mux.Handle(DevicePath, s)
	mux.Handle(MediaPath, s)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	dev, media := srv.URL+DevicePath, srv.URL+MediaPath
	host := strings.TrimPrefix(srv.URL, "http://")
	name := host[:strings.LastIndex(host, ":")]

	for _, c := range []struct {
		url, op, inner string
		want           []string
	}{
		{dev, "tds:GetSystemDateAndTime", "", []string{"<tt:Year>2026</tt:Year>", "<tt:Hour>13</tt:Hour>"}},
		{dev, "tds:GetDeviceInformation", "", []string{"<tds:Manufacturer>Lenovo</tds:Manufacturer>", "<tds:SerialNumber>HUA03RW0</tds:SerialNumber>"}},
		{dev, "tds:GetCapabilities", "", []string{srv.URL + MediaPath, "<tt:RTP_RTSP_TCP>true</tt:RTP_RTSP_TCP>"}},
		{dev, "tds:GetServices", "<tds:IncludeCapability>false</tds:IncludeCapability>", []string{"http://www.onvif.org/ver10/media/wsdl"}},
		{dev, "tds:GetNetworkInterfaces", "", []string{"aa:bb:cc:dd:ee:ff"}},
		{media, "trt:GetProfiles", "", []string{`token="main"`, `token="sub"`, "<tt:Width>1280</tt:Width>", "<tt:Encoding>H264</tt:Encoding>"}},
		{media, "trt:GetStreamUri", "<trt:ProfileToken>sub</trt:ProfileToken>", []string{"rtsp://" + name + ":8554/sub"}},
		{media, "trt:GetProfile", "<trt:ProfileToken>main</trt:ProfileToken>", []string{"<tt:Width>2592</tt:Width>"}},
	} {
		code, body := soap(t, c.url, c.op, c.inner)
		if code != 200 {
			t.Errorf("%s: %d\n%s", c.op, code, body)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(body, w) {
				t.Errorf("%s lacks %q:\n%s", c.op, w, body)
			}
		}
	}

	code, body := soap(t, dev, "tds:SetHostname", "<tds:Name>x</tds:Name>")
	if code != 500 || !strings.Contains(body, "ActionNotSupported") {
		t.Errorf("an unsupported action answered %d:\n%s", code, body)
	}
	if code, _ := soap(t, media, "trt:GetSnapshotUri", "<trt:ProfileToken>main</trt:ProfileToken>"); code != 500 {
		t.Errorf("a snapshot with no snapshot path answered %d", code)
	}
}
