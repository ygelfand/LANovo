package primevideo

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	PlaybackHost = "https://aby4wfamebrp.api.amazonvideo.com"
	ZazHost      = "https://aby4wfamebrp.zaz.api.amazonvideo.com"
	firmware     = "1.56.500000"
	deviceModel  = "SHIELD Android TV"
)

type Account struct {
	Token       string
	Device      string
	Marketplace string
	Locale      string
}

type Screen struct {
	Width, Height int
	Tallest       int
	Codecs        []string
	MaxResolution string
}

type Resources struct {
	Handoff string
	Default string
	URLs    []URLSet
}

type URLSet struct {
	ID  string `json:"urlSetId"`
	URL string `json:"url"`
}

func nerid() string {
	var b [8]byte
	rand.Read(b[:])
	return "lanovo" + hex.EncodeToString(b[:])
}

func address(host, path string, query [][2]string) string {
	v := url.Values{}
	for _, q := range query {
		v.Add(q[0], q[1])
	}
	return host + path + "?" + v.Encode()
}

func RefreshEnvelope(ctx context.Context, hc *http.Client, a Account, title, correlation string) (Envelope, error) {
	target := address(PlaybackHost, "/playback/tags/getRefreshedPlaybackEnvelope", [][2]string{
		{"deviceID", a.Device}, {"deviceTypeID", DeviceType}, {"gascEnabled", "true"},
		{"marketplaceID", a.Marketplace}, {"firmware", "1"}, {"version", "1"}, {"nerid", nerid()},
	})
	body := map[string]any{
		"deviceId": a.Device, "deviceTypeId": DeviceType,
		"identifiers": map[string]string{title: correlation},
		"geoToken":    nil, "identityContext": nil,
	}
	var out struct {
		Response map[string]struct {
			Experience *struct {
				Correlation string `json:"correlationId"`
				Envelope    string `json:"playbackEnvelope"`
			} `json:"playbackExperience"`
		} `json:"response"`
	}
	if err := postText(ctx, hc, target, a.Token, body, &out); err != nil {
		return Envelope{}, fmt.Errorf("prime video: refresh envelope: %w", err)
	}
	e := out.Response[title].Experience
	if e == nil || e.Envelope == "" {
		return Envelope{}, errors.New("prime video: refresh envelope: none returned")
	}
	return Envelope{Envelope: e.Envelope, Correlation: e.Correlation}, nil
}

func VodResources(ctx context.Context, hc *http.Client, a Account, s Screen, title, envelope string) (Resources, error) {
	target := address(ZazHost, "/playback/prs/GetVodPlaybackResources", [][2]string{
		{"deviceID", a.Device}, {"deviceTypeID", DeviceType}, {"gascEnabled", "true"},
		{"marketplaceID", a.Marketplace}, {"uxLocale", a.Locale}, {"firmware", "1"},
		{"titleId", title}, {"nerid", nerid()},
	})
	body := map[string]any{
		"globalParameters": map[string]any{
			"deviceCapabilityFamily": "WebPlayer",
			"playbackEnvelope":       envelope,
			"capabilityDiscriminators": map[string]any{
				"operatingSystem":   map[string]string{"name": "Android", "version": "11.0"},
				"deviceModel":       map[string]string{"name": deviceModel, "version": "UNKNOWN"},
				"middleware":        map[string]string{"name": "Chrome", "version": "92.0.4515.0"},
				"nativeApplication": map[string]string{"name": "CAF Receiver SDK", "version": "3.0.0137"},
				"firmware":          map[string]string{"name": "UNKNOWN", "version": firmware},
				"hfrControlMode":    "Legacy",
				"displayResolution": map[string]int{"height": s.Height, "width": s.Width},
			},
		},
		"auditPingsRequest":                 map[string]any{},
		"widevineServiceCertificateRequest": map[string]any{},
		"playbackDataRequest":               map[string]any{},
		"timedTextUrlsRequest":              map[string]any{"supportedTimedTextFormats": []string{"TTMLv2", "DFXP"}},
		"trickplayUrlsRequest":              map[string]any{},
		"transitionTimecodesRequest":        map[string]any{},
		"vodPlaybackUrlsRequest": map[string]any{
			"device": map[string]any{
				"hdcpLevel":                      "1.4",
				"maxVideoResolution":             s.MaxResolution,
				"supportedStreamingTechnologies": []string{"DASH"},
				"streamingTechnologies": map[string]any{"DASH": map[string]any{
					"bitrateAdaptations":               []string{"CBR", "CVBR"},
					"codecs":                           s.Codecs,
					"drmKeyScheme":                     "DualKey",
					"drmType":                          "Widevine",
					"dynamicRangeFormats":              []string{"None"},
					"edgeDeliveryAuthorizationSchemes": []string{"PVExchangeV1", "Transparent"},
					"fragmentRepresentations":          []string{"ByteOffsetRange", "SeparateFile"},
					"frameRates":                       []string{"Standard"},
					"stitchType":                       "MultiPeriod",
					"segmentInfoType":                  "Base",
					"timedTextRepresentations":         []string{"NotInManifestNorStream", "SeparateStreamInManifest"},
					"trickplayRepresentations":         []string{"NotInManifestNorStream"},
					"variableAspectRatio":              "unsupported",
				}},
				"displayWidth":  s.Width,
				"displayHeight": s.Height,
			},
			"ads": map[string]any{
				"sitePageUrl":                       "https://cloudfront.xp-assets.aiv-cdn.net/packages/ATVGCastReceiver-1.0/prod/index.html",
				"gdpr":                              map[string]any{"enabled": false, "consentMap": map[string]any{}},
				"mainContentResumeOffsetHintMillis": 0,
			},
			"playbackCustomizations": map[string]any{},
			"playbackSettingsRequest": map[string]string{
				"deviceModel": deviceModel, "firmware": firmware, "playerType": "xp",
				"responseFormatVersion": "1.0.0", "titleId": title,
			},
		},
		"vodXrayMetadataRequest": map[string]string{
			"xrayDeviceClass": "normal", "xrayPlaybackMode": "playback", "xrayToken": "XRAY_WEB_2023_V2",
		},
	}
	var out struct {
		Session *struct {
			Handoff string `json:"sessionHandoffToken"`
		} `json:"sessionization"`
		Vod *struct {
			Result *struct {
				URLs *struct {
					Default string   `json:"defaultUrlSetId"`
					Sets    []URLSet `json:"urlSets"`
				} `json:"playbackUrls"`
			} `json:"result"`
			Error json.RawMessage `json:"error"`
		} `json:"vodPlaybackUrls"`
	}
	if err := postText(ctx, hc, target, a.Token, body, &out); err != nil {
		return Resources{}, fmt.Errorf("prime video: playback resources: %w", err)
	}
	if out.Vod == nil || out.Vod.Result == nil || out.Vod.Result.URLs == nil || len(out.Vod.Result.URLs.Sets) == 0 {
		var why string
		if out.Vod != nil {
			why = string(out.Vod.Error)
		}
		return Resources{}, fmt.Errorf("prime video: playback resources: no urls: %.400s", why)
	}
	r := Resources{Default: out.Vod.Result.URLs.Default, URLs: out.Vod.Result.URLs.Sets}
	if out.Session != nil {
		r.Handoff = out.Session.Handoff
	}
	return r, nil
}

func postText(ctx context.Context, hc *http.Client, target, token string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("status %d: %.400s", resp.StatusCode, data)
	}
	return json.Unmarshal(data, out)
}

func Manifest(ctx context.Context, hc *http.Client, target string) ([]byte, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if q.Get("amznDtid") == "" {
		q.Set("amznDtid", DeviceType)
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("prime video: manifest status %d: %.300s", resp.StatusCode, data)
	}
	return data, nil
}

func License(ctx context.Context, hc *http.Client, a Account, title, envelope, handoff string, challenge []byte) ([]byte, error) {
	target := address(ZazHost, "/playback/drm-vod/GetWidevineLicense", [][2]string{
		{"deviceID", a.Device}, {"deviceTypeID", DeviceType}, {"gascEnabled", "true"},
		{"marketplaceID", a.Marketplace}, {"uxLocale", a.Locale}, {"firmware", "1"},
		{"titleId", title}, {"nerid", nerid()},
	})
	body := map[string]any{
		"includeHdcpTestKey": true,
		"playbackEnvelope":   envelope,
		"licenseChallenge":   base64.StdEncoding.EncodeToString(challenge),
	}
	if handoff != "" {
		body["sessionHandoffToken"] = handoff
	}
	var out struct {
		Widevine struct {
			License string `json:"license"`
		} `json:"widevineLicense"`
		Error json.RawMessage `json:"error"`
	}
	if err := postText(ctx, hc, target, a.Token, body, &out); err != nil {
		return nil, fmt.Errorf("prime video: license: %w", err)
	}
	if out.Widevine.License == "" {
		return nil, fmt.Errorf("prime video: license: none returned: %.300s", out.Error)
	}
	lic, err := base64.StdEncoding.DecodeString(out.Widevine.License)
	if err != nil {
		lic, err = base64.RawStdEncoding.DecodeString(strings.TrimRight(out.Widevine.License, "="))
	}
	return lic, err
}

func Fetch(ctx context.Context, hc *http.Client, target string, from, to int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", from, to))
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return nil, fmt.Errorf("prime video: fetch status %d: %s", resp.StatusCode, data)
	}
	return io.ReadAll(io.LimitReader(resp.Body, to-from+1))
}

func Catalog(ctx context.Context, hc *http.Client, a Account, title string) (name, series string, err error) {
	target := address(PlaybackHost, "/cdp/lumina/playerChromeResources/v1", [][2]string{
		{"deviceID", a.Device}, {"deviceTypeID", DeviceType}, {"gascEnabled", "true"},
		{"marketplaceID", a.Marketplace}, {"uxLocale", a.Locale}, {"desiredResources", "catalogMetadataV2"},
		{"entityId", title}, {"firmware", "1"}, {"widgetScheme", "pvplayer-web-v2"}, {"nerid", nerid()},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")
	req.Header.Set("Authorization", "Bearer "+a.Token)
	resp, err := hc.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("prime video: catalog status %d: %.300s", resp.StatusCode, data)
	}
	var out struct {
		Resources struct {
			Catalog struct {
				Catalog struct {
					Title  string `json:"title"`
					Event  string `json:"eventTitle"`
					Series string `json:"seriesTitle"`
				} `json:"catalog"`
			} `json:"catalogMetadataV2"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", "", fmt.Errorf("prime video: catalog: %w", err)
	}
	c := out.Resources.Catalog.Catalog
	return first(c.Event, c.Title), c.Series, nil
}
