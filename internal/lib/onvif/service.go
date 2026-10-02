package onvif

import (
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	DevicePath = "/onvif/device_service"
	MediaPath  = "/onvif/media_service"
)

// Profile is one stream, as a media profile names it.
type Profile struct {
	Token         string
	Name          string
	Width, Height int
	FPS           int
	Bitrate       int
	Path          string
}

// Device is what the service says about the camera.
type Device struct {
	Manufacturer string
	Model        string
	Firmware     string
	Serial       string
	Hardware     string
	MAC          string
	Name         string
	RTSPPort     int
	Profiles     func() []Profile
	SnapshotPath string
}

// Service answers the device and media services.
type Service struct {
	Device Device
	Now    func() time.Time
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "SOAP over POST", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	op, args := operation(body)
	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	reply, ok := s.answer(op, args, r.Host)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, envelope(fault(op)))
		return
	}
	io.WriteString(w, envelope(reply))
}

type arg struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

func operation(body []byte) (string, map[string]string) {
	var env struct {
		Body struct {
			Inner struct {
				XMLName xml.Name
				Args    []arg `xml:",any"`
			} `xml:",any"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(body, &env); err != nil {
		return "", nil
	}
	args := map[string]string{}
	for _, a := range env.Body.Inner.Args {
		args[a.XMLName.Local] = strings.TrimSpace(a.Value)
	}
	return env.Body.Inner.XMLName.Local, args
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) profiles() []Profile {
	if s.Device.Profiles == nil {
		return nil
	}
	return s.Device.Profiles()
}

func (s *Service) answer(op string, args map[string]string, host string) (string, bool) {
	d := s.Device
	name, _, err := net.SplitHostPort(host)
	if err != nil {
		name = host
	}
	base := "http://" + host
	switch op {
	case "GetSystemDateAndTime":
		t := s.now().UTC()
		return fmt.Sprintf(`<tds:GetSystemDateAndTimeResponse><tds:SystemDateAndTime>`+
			`<tt:DateTimeType>NTP</tt:DateTimeType><tt:DaylightSavings>false</tt:DaylightSavings>`+
			`<tt:TimeZone><tt:TZ>UTC0</tt:TZ></tt:TimeZone><tt:UTCDateTime>`+
			`<tt:Time><tt:Hour>%d</tt:Hour><tt:Minute>%d</tt:Minute><tt:Second>%d</tt:Second></tt:Time>`+
			`<tt:Date><tt:Year>%d</tt:Year><tt:Month>%d</tt:Month><tt:Day>%d</tt:Day></tt:Date>`+
			`</tt:UTCDateTime></tds:SystemDateAndTime></tds:GetSystemDateAndTimeResponse>`,
			t.Hour(), t.Minute(), t.Second(), t.Year(), int(t.Month()), t.Day()), true
	case "GetDeviceInformation":
		return fmt.Sprintf(`<tds:GetDeviceInformationResponse><tds:Manufacturer>%s</tds:Manufacturer>`+
			`<tds:Model>%s</tds:Model><tds:FirmwareVersion>%s</tds:FirmwareVersion>`+
			`<tds:SerialNumber>%s</tds:SerialNumber><tds:HardwareId>%s</tds:HardwareId>`+
			`</tds:GetDeviceInformationResponse>`,
			escape(d.Manufacturer), escape(d.Model), escape(d.Firmware), escape(d.Serial), escape(d.Hardware)), true
	case "GetCapabilities":
		return fmt.Sprintf(`<tds:GetCapabilitiesResponse><tds:Capabilities>`+
			`<tt:Device><tt:XAddr>%s%s</tt:XAddr></tt:Device>`+
			`<tt:Media><tt:XAddr>%s%s</tt:XAddr><tt:StreamingCapabilities>`+
			`<tt:RTPMulticast>false</tt:RTPMulticast><tt:RTP_TCP>true</tt:RTP_TCP><tt:RTP_RTSP_TCP>true</tt:RTP_RTSP_TCP>`+
			`</tt:StreamingCapabilities></tt:Media></tds:Capabilities></tds:GetCapabilitiesResponse>`,
			base, DevicePath, base, MediaPath), true
	case "GetServices":
		return fmt.Sprintf(`<tds:GetServicesResponse>`+
			`<tds:Service><tds:Namespace>http://www.onvif.org/ver10/device/wsdl</tds:Namespace><tds:XAddr>%s%s</tds:XAddr>`+
			`<tds:Version><tt:Major>2</tt:Major><tt:Minor>40</tt:Minor></tds:Version></tds:Service>`+
			`<tds:Service><tds:Namespace>http://www.onvif.org/ver10/media/wsdl</tds:Namespace><tds:XAddr>%s%s</tds:XAddr>`+
			`<tds:Version><tt:Major>2</tt:Major><tt:Minor>40</tt:Minor></tds:Version></tds:Service>`+
			`</tds:GetServicesResponse>`, base, DevicePath, base, MediaPath), true
	case "GetScopes":
		var b strings.Builder
		b.WriteString(`<tds:GetScopesResponse>`)
		for _, sc := range []string{"type/video_encoder", "Profile/Streaming", "name/" + scopeName(d.Name), "hardware/" + scopeName(d.Model)} {
			fmt.Fprintf(&b, `<tds:Scopes><tt:ScopeDef>Fixed</tt:ScopeDef><tt:ScopeItem>onvif://www.onvif.org/%s</tt:ScopeItem></tds:Scopes>`, sc)
		}
		b.WriteString(`</tds:GetScopesResponse>`)
		return b.String(), true
	case "GetNetworkInterfaces":
		return fmt.Sprintf(`<tds:GetNetworkInterfacesResponse><tds:NetworkInterfaces token="eth0">`+
			`<tt:Enabled>true</tt:Enabled><tt:Info><tt:Name>wlan0</tt:Name><tt:HwAddress>%s</tt:HwAddress></tt:Info>`+
			`</tds:NetworkInterfaces></tds:GetNetworkInterfacesResponse>`, escape(d.MAC)), true
	case "GetHostname":
		return fmt.Sprintf(`<tds:GetHostnameResponse><tds:HostnameInformation><tt:FromDHCP>true</tt:FromDHCP>`+
			`<tt:Name>%s</tt:Name></tds:HostnameInformation></tds:GetHostnameResponse>`, escape(d.Name)), true
	case "GetProfiles":
		var b strings.Builder
		b.WriteString(`<trt:GetProfilesResponse>`)
		for _, p := range s.profiles() {
			b.WriteString(profileXML("trt:Profiles", p))
		}
		b.WriteString(`</trt:GetProfilesResponse>`)
		return b.String(), true
	case "GetProfile":
		for _, p := range s.profiles() {
			if p.Token == args["ProfileToken"] {
				return `<trt:GetProfileResponse>` + profileXML("trt:Profile", p) + `</trt:GetProfileResponse>`, true
			}
		}
		return "", false
	case "GetVideoSources":
		var b strings.Builder
		b.WriteString(`<trt:GetVideoSourcesResponse>`)
		if ps := s.profiles(); len(ps) > 0 {
			fmt.Fprintf(&b, `<trt:VideoSources token="source"><tt:Framerate>%d</tt:Framerate>`+
				`<tt:Resolution><tt:Width>%d</tt:Width><tt:Height>%d</tt:Height></tt:Resolution></trt:VideoSources>`,
				ps[0].FPS, ps[0].Width, ps[0].Height)
		}
		b.WriteString(`</trt:GetVideoSourcesResponse>`)
		return b.String(), true
	case "GetStreamUri":
		for _, p := range s.profiles() {
			if p.Token == args["ProfileToken"] || args["ProfileToken"] == "" {
				uri := fmt.Sprintf("rtsp://%s/%s", net.JoinHostPort(name, fmt.Sprint(d.RTSPPort)), strings.TrimPrefix(p.Path, "/"))
				return fmt.Sprintf(`<trt:GetStreamUriResponse><trt:MediaUri><tt:Uri>%s</tt:Uri>`+
					`<tt:InvalidAfterConnect>false</tt:InvalidAfterConnect><tt:InvalidAfterReboot>false</tt:InvalidAfterReboot>`+
					`<tt:Timeout>PT0S</tt:Timeout></trt:MediaUri></trt:GetStreamUriResponse>`, escape(uri)), true
			}
		}
		return "", false
	case "GetSnapshotUri":
		if d.SnapshotPath == "" {
			return "", false
		}
		return fmt.Sprintf(`<trt:GetSnapshotUriResponse><trt:MediaUri><tt:Uri>%s%s</tt:Uri>`+
			`<tt:InvalidAfterConnect>false</tt:InvalidAfterConnect><tt:InvalidAfterReboot>false</tt:InvalidAfterReboot>`+
			`<tt:Timeout>PT0S</tt:Timeout></trt:MediaUri></trt:GetSnapshotUriResponse>`, base, escape(d.SnapshotPath)), true
	}
	return "", false
}

func profileXML(tag string, p Profile) string {
	return fmt.Sprintf(`<%s token="%s" fixed="true"><tt:Name>%s</tt:Name>`+
		`<tt:VideoSourceConfiguration token="source"><tt:Name>source</tt:Name><tt:UseCount>1</tt:UseCount>`+
		`<tt:SourceToken>source</tt:SourceToken><tt:Bounds x="0" y="0" width="%d" height="%d"/></tt:VideoSourceConfiguration>`+
		`<tt:VideoEncoderConfiguration token="%s_enc"><tt:Name>%s</tt:Name><tt:UseCount>1</tt:UseCount>`+
		`<tt:Encoding>H264</tt:Encoding><tt:Resolution><tt:Width>%d</tt:Width><tt:Height>%d</tt:Height></tt:Resolution>`+
		`<tt:Quality>5</tt:Quality><tt:RateControl><tt:FrameRateLimit>%d</tt:FrameRateLimit><tt:EncodingInterval>1</tt:EncodingInterval>`+
		`<tt:BitrateLimit>%d</tt:BitrateLimit></tt:RateControl><tt:H264><tt:GovLength>%d</tt:GovLength><tt:H264Profile>Baseline</tt:H264Profile></tt:H264>`+
		`<tt:SessionTimeout>PT60S</tt:SessionTimeout></tt:VideoEncoderConfiguration></%s>`,
		tag, escape(p.Token), escape(p.Name), p.Width, p.Height,
		escape(p.Token), escape(p.Name), p.Width, p.Height, p.FPS, p.Bitrate/1000, max(p.FPS*2, 1), tag)
}

func envelope(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:tt="http://www.onvif.org/ver10/schema" ` +
		`xmlns:tds="http://www.onvif.org/ver10/device/wsdl" xmlns:trt="http://www.onvif.org/ver10/media/wsdl" ` +
		`xmlns:ter="http://www.onvif.org/ver10/error">` +
		`<s:Body>` + body + `</s:Body></s:Envelope>`
}

func fault(op string) string {
	return `<s:Fault><s:Code><s:Value>s:Sender</s:Value><s:Subcode><s:Value>ter:ActionNotSupported</s:Value></s:Subcode></s:Code>` +
		`<s:Reason><s:Text xml:lang="en">` + escape(op) + ` is not supported</s:Text></s:Reason></s:Fault>`
}
