package cast

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NSSetup carries eureka_info, the device description the setup HTTP endpoint also serves.
const NSSetup = "urn:x-cast:com.google.cast.setup"

// TypeEurekaInfo asks for the device description.
const TypeEurekaInfo = "eureka_info"

// What a stock Lenovo Smart Display 10 on 1.56.285116 reports.
const (
	EurekaVersion = 12
	CastBuild     = "1.56.285116"
	BuildVersion  = "285116"
	BuildType     = 2
)

// SetupDone is setup_state once setup has finished.
const SetupDone = 60

// Eureka is the device description, the non-personal subset of what a stock device reports.
type Eureka struct {
	Name              string  `json:"name"`
	Version           int     `json:"version"`
	BuildVersion      string  `json:"build_version"`
	CastBuildRevision string  `json:"cast_build_revision"`
	UDN               string  `json:"ssdp_udn"`
	IP                string  `json:"ip_address,omitempty"`
	Locale            string  `json:"locale"`
	Timezone          string  `json:"timezone,omitempty"`
	Connected         bool    `json:"connected"`
	Ethernet          bool    `json:"ethernet_connected"`
	HasUpdate         bool    `json:"has_update"`
	SetupState        int     `json:"setup_state"`
	TOSAccepted       bool    `json:"tos_accepted"`
	Uptime            float64 `json:"uptime"`
	OptIn             OptIn   `json:"opt_in"`

	DeviceInfo DeviceInfo `json:"device_info"`
	BuildInfo  BuildInfo  `json:"build_info"`
	Multizone  Multizone  `json:"multizone"`
}

type OptIn struct {
	Crash    bool `json:"crash"`
	Opencast bool `json:"opencast"`
	Stats    bool `json:"stats"`
}

type DeviceInfo struct {
	CloudDeviceID string `json:"cloud_device_id"`
	Manufacturer  string `json:"manufacturer"`
	ProductName   string `json:"product_name"`
	UDN           string `json:"ssdp_udn"`
}

type BuildInfo struct {
	BuildType         int    `json:"build_type"`
	CastBuildRevision string `json:"cast_build_revision"`
	SystemBuildNumber string `json:"system_build_number"`
}

type Multizone struct {
	AudioOutputDelay     float64 `json:"audio_output_delay"`
	AudioOutputDelayHDMI float64 `json:"audio_output_delay_hdmi"`
	AudioOutputDelayOEM  float64 `json:"audio_output_delay_oem"`
	AuxInGroup           string  `json:"aux_in_group"`
	DynamicGroups        []any   `json:"dynamic_groups"`
	Groups               []any   `json:"groups"`
	MultichannelStatus   int     `json:"multichannel_status"`
}

// groups are the nested parts, which a stock device only returns when they are asked for by path.
var groups = []string{"device_info", "build_info", "multizone"}

// NewEureka is the description of a device that has finished setup, with the cast build a stock one
// has.
func NewEureka(d Device, manufacturer, system string) Eureka {
	return Eureka{
		Name:              d.Name,
		Version:           EurekaVersion,
		BuildVersion:      BuildVersion,
		CastBuildRevision: CastBuild,
		UDN:               d.UDN(),
		Locale:            "en-US",
		Connected:         true,
		SetupState:        SetupDone,
		TOSAccepted:       true,
		DeviceInfo: DeviceInfo{
			CloudDeviceID: d.CloudID(),
			Manufacturer:  manufacturer,
			ProductName:   d.Model,
			UDN:           d.UDN(),
		},
		BuildInfo: BuildInfo{
			BuildType:         BuildType,
			CastBuildRevision: CastBuild,
			SystemBuildNumber: system,
		},
		Multizone: Multizone{DynamicGroups: []any{}, Groups: []any{}},
	}
}

// Select is the description as a stock device answers: every flat field when nothing is named, and
// only the named dotted paths, nested, when something is. A path it does not have is left out.
func (e Eureka) Select(params []string) map[string]any {
	raw, _ := json.Marshal(e)
	var all map[string]any
	json.Unmarshal(raw, &all)

	if len(params) == 0 {
		for _, g := range groups {
			delete(all, g)
		}
		return all
	}

	out := map[string]any{}
	for _, p := range params {
		copyPath(all, out, strings.Split(p, "."))
	}
	return out
}

func copyPath(from, to map[string]any, path []string) {
	v, ok := from[path[0]]
	if !ok {
		return
	}
	if len(path) == 1 {
		to[path[0]] = v
		return
	}
	sub, ok := v.(map[string]any)
	if !ok {
		return
	}
	next, ok := to[path[0]].(map[string]any)
	if !ok {
		next = map[string]any{}
	}
	copyPath(sub, next, path[1:])
	if len(next) > 0 {
		to[path[0]] = next
	}
}

// UDN is the device id as a dashed UUID, which is how ssdp_udn and the mDNS host spell it.
func (d Device) UDN() string {
	if len(d.ID) != 32 {
		return d.ID
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", d.ID[0:8], d.ID[8:12], d.ID[12:16], d.ID[16:20], d.ID[20:32])
}

type setupRequest struct {
	Type      string          `json:"type"`
	RequestID int             `json:"request_id"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type setupResponse struct {
	Type           string `json:"type"`
	RequestID      int    `json:"request_id"`
	ResponseCode   int    `json:"response_code"`
	ResponseString string `json:"response_string"`
	Data           any    `json:"data"`
}

// params reads the paths a request names, as a list or as the comma-separated string the HTTP
// endpoint takes.
func params(data json.RawMessage) []string {
	var asList struct {
		Params []string `json:"params"`
	}
	if json.Unmarshal(data, &asList) == nil {
		return asList.Params
	}
	var asString struct {
		Params string `json:"params"`
	}
	if json.Unmarshal(data, &asString) == nil && asString.Params != "" {
		return strings.Split(asString.Params, ",")
	}
	return nil
}

func (r *Receiver) setup(m Message) ([]Message, error) {
	var req setupRequest
	if err := json.Unmarshal([]byte(m.Payload), &req); err != nil {
		return nil, fmt.Errorf("cast: a setup message: %w", err)
	}
	if r.Setup != nil {
		r.Setup(req.Type, req.Data)
	}
	if req.Type != TypeEurekaInfo {
		if r.Unspoken != nil {
			r.Unspoken(m.Namespace, req.Type)
		}
		return nil, nil
	}

	var data any = struct{}{}
	if r.Eureka != nil {
		data = r.Eureka().Select(params(req.Data))
	}
	body, err := json.Marshal(setupResponse{
		Type:           TypeEurekaInfo,
		RequestID:      req.RequestID,
		ResponseCode:   200,
		ResponseString: "OK",
		Data:           data,
	})
	if err != nil {
		return nil, err
	}
	return []Message{reply(m, NSSetup, string(body))}, nil
}
