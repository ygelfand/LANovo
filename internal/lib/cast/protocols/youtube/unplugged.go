package youtube

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/lib/cast/playback"
	"github.com/ygelfand/LANovo/internal/lib/cenc"
	"github.com/ygelfand/LANovo/internal/lib/surface"
)

var (
	unpluggedPlayer = "https://youtubei.googleapis.com/youtubei/v1/player?key=AIzaSyBoV7Fjr-XX5PO6SchLPINEMLkSkXVOh5M&prettyPrint=false"
	licenseURL      = "https://tv.youtube.com/youtubei/v1/player/get_drm_license?alt=json"
	guideURL        = "https://tv.youtube.com/youtubei/v1/next?prettyPrint=false"
	heartbeatURL    = "https://tv.youtube.com/youtubei/v1/player/heartbeat?alt=json"
)

const (
	unpluggedClient  = "IOS_UNPLUGGED"
	unpluggedVersion = "10.39.0"
	unpluggedAgent   = "com.google.ios.youtubeunplugged/10.39.0 (iPhone16,2; U; CPU iOS 18_6 like Mac OS X)"

	licenseOrigin  = "https://tv.youtube.com"
	licenseClient  = "WEB_UNPLUGGED"
	licenseVersion = "1.20260928.04.00"
	licenseNumber  = "41"

	// Google's license.widevine.com service certificate, as tv.youtube.com's player serves it.
	widevineCert = "CsECCAMSEBcFuRfMEgSGiwYzOi93KowYgrSCkgUijgIwggEKAoIBAQCZ7Vs7Mn2rXiTvw7YqlbWYUgrVvMs3UD4GRbgU2Ha430BRBEGtjOOtsRu4jE5yWl5KngeVKR1YWEAjp-GvDjipEnk5MAhhC28VjIeMfiG_-_7qd-EBnh5XgeikX0YmPRTmDoBYqGB63OBPrIRXsTeo1nzN6zNwXZg6IftO7L1KEMpHSQykfqpdQ4IY3brxyt4zkvE9b_tkQv0x4b9AsMYE0cS6TJUgpL-X7r1gkpr87vVbuvVk4tDnbNfFXHOggrmWEguDWe3OJHBwgmgNb2fG2CxKxfMTRJCnTuw3r0svAQxZ6ChD4lgvC2ufXbD8Xm7fZPvTCLRxG88SUAGcn1oJAgMBAAE6FGxpY2Vuc2Uud2lkZXZpbmUuY29tEoADrjRzFLWoNSl_JxOI-3u4y1J30kmCPN3R2jC5MzlRHrPMveoEuUS5J8EhNG79verJ1BORfm7BdqEEOEYKUDvBlSubpOTOD8S_wgqYCKqvS_zRnB3PzfV0zKwo0bQQQWz53ogEMBy9szTK_NDUCXhCOmQuVGE98K_PlspKkknYVeQrOnA-8XZ_apvTbWv4K-drvwy6T95Z0qvMdv62Qke4XEMfvKUiZrYZ_DaXlUP8qcu9u_r6DhpV51Wjx7zmVflkb1gquc9wqgi5efhn9joLK3_bNixbxOzVVdhbyqnFk8ODyFfUnaq3fkC3hR3f0kmYgI41sljnXXjqwMoW9wRzBMINk-3k6P8cbxfmJD4_Paj8FwmHDsRfuoI6Jj8M76H3CTsZCZKDJjM3BQQ6Kb2m-bQ0LMjfVDyxoRgvfF__M_EEkPrKWyU2C3YBXpxaBquO4C8A0ujVmGEEqsxN1HX9lu6c5OMm8huDxwWFd7OHMs3avGpr7RP7DUnTikXrh6X0"

	unpluggedTallest = 720
	segmentWait      = time.Second
	heldSeconds      = 30
	tvStartWait      = 20 * time.Second
	segmentLimit     = 32 << 20
)

var drmIDs, audioIDs atomic.Uint32

type unpluggedFormat struct {
	Itag       int    `json:"itag"`
	URL        string `json:"url"`
	MimeType   string `json:"mimeType"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	FPS        int    `json:"fps"`
	Bitrate    int    `json:"bitrate"`
	AudioTrack *struct {
		AudioIsDefault bool `json:"audioIsDefault"`
	} `json:"audioTrack"`
}

type unpluggedAnswer struct {
	PlayabilityStatus struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"playabilityStatus"`
	StreamingData struct {
		AdaptiveFormats []unpluggedFormat `json:"adaptiveFormats"`
		DRMParams       string            `json:"drmParams"`
	} `json:"streamingData"`
	HeartbeatParams struct {
		DRMSessionID string `json:"drmSessionId"`
		Token        string `json:"heartbeatToken"`
		ServerData   string `json:"heartbeatServerData"`
		Interval     string `json:"intervalMilliseconds"`
	} `json:"heartbeatParams"`
	VideoDetails struct {
		Title     string `json:"title"`
		Author    string `json:"author"`
		Thumbnail struct {
			Thumbnails []struct {
				URL string `json:"url"`
			} `json:"thumbnails"`
		} `json:"thumbnail"`
	} `json:"videoDetails"`
	PlayerConfig struct {
		Live struct {
			MinDvrSequence string `json:"minDvrSequence"`
		} `json:"manifestlessWindowedLiveConfig"`
	} `json:"playerConfig"`
}

func credential(ctt string) map[string]any {
	return map[string]any{"credentialTransferTokens": []map[string]string{{"token": ctt, "scope": "VIDEO"}}}
}

func postJSON(ctx context.Context, hc *http.Client, target string, body any, headers map[string]string, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", resp.Status, bytes.TrimSpace(data[:min(len(data), 200)]))
	}
	return json.Unmarshal(data, out)
}

func unpluggedPlay(ctx context.Context, hc *http.Client, ctt, id, params string) (*unpluggedAnswer, error) {
	body := map[string]any{
		"videoId": id,
		"params":  params,
		"context": map[string]any{
			"client":  map[string]string{"hl": "en", "gl": "US", "clientName": unpluggedClient, "clientVersion": unpluggedVersion},
			"user":    credential(ctt),
			"request": map[string]bool{"useSsl": true},
		},
		"playbackContext": map[string]any{"contentPlaybackContext": map[string]string{"html5Preference": "HTML5_PREF_WANTS"}},
		"racyCheckOk":     true,
	}
	var a unpluggedAnswer
	err := postJSON(ctx, hc, unpluggedPlayer, body, map[string]string{"User-Agent": unpluggedAgent, "X-Youtube-Client-Version": unpluggedVersion}, &a)
	if err != nil {
		return nil, fmt.Errorf("youtube tv: player: %w", err)
	}
	if a.PlayabilityStatus.Status != "OK" {
		return nil, fmt.Errorf("youtube tv: %s: %s", a.PlayabilityStatus.Status, a.PlayabilityStatus.Reason)
	}
	return &a, nil
}

func pickUnplugged(fs []unpluggedFormat) (video, audio *unpluggedFormat) {
	for i := range fs {
		f := &fs[i]
		switch {
		case strings.HasPrefix(f.MimeType, "video/mp4") && strings.Contains(f.MimeType, "avc1") && f.Height <= unpluggedTallest && f.FPS <= fastest:
			if video == nil || f.Height > video.Height || f.Height == video.Height && f.Bitrate > video.Bitrate {
				video = f
			}
		case strings.HasPrefix(f.MimeType, "audio/mp4") && strings.Contains(f.MimeType, "mp4a.40.2"):
			def := f.AudioTrack == nil || f.AudioTrack.AudioIsDefault
			if def && (audio == nil || f.Bitrate > audio.Bitrate) {
				audio = f
			}
		}
	}
	return video, audio
}

func nonce() string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	b := make([]byte, 16)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[b[i]&63]
	}
	return string(b)
}

func unpluggedLicense(ctx context.Context, hc *http.Client, ctt, id, cpn, session, params string, challenge []byte) ([]byte, error) {
	pdrm, _ := url.QueryUnescape(params)
	body := map[string]any{
		"context": map[string]any{
			"client": map[string]string{"hl": "en", "gl": "US", "clientName": licenseClient, "clientVersion": licenseVersion},
			"user":   credential(ctt),
		},
		"drmSystem":         "DRM_SYSTEM_WIDEVINE",
		"videoId":           id,
		"cpn":               cpn,
		"sessionId":         session,
		"licenseRequest":    base64.StdEncoding.EncodeToString(challenge),
		"drmParams":         pdrm,
		"isKeyRotated":      true,
		"cryptoPeriodIndex": time.Now().Unix() / 86400,
		"drmVideoFeature":   "DRM_VIDEO_FEATURE_SDR",
	}
	var a struct {
		Status  string `json:"status"`
		License string `json:"license"`
	}
	err := postJSON(ctx, hc, licenseURL, body, map[string]string{
		"Origin": licenseOrigin, "X-Origin": licenseOrigin,
		"X-Youtube-Client-Name": licenseNumber, "X-Youtube-Client-Version": licenseVersion,
	}, &a)
	if err != nil {
		return nil, fmt.Errorf("youtube tv: license: %w", err)
	}
	if a.Status != "LICENSE_STATUS_OK" || a.License == "" {
		return nil, fmt.Errorf("youtube tv: license %s", a.Status)
	}
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(a.License, "="))
}

const (
	airingRetry = 5 * time.Minute
	airingSlack = 5 * time.Second
)

type textRuns struct {
	Runs []struct {
		Text string `json:"text"`
	} `json:"runs"`
	SimpleText string `json:"simpleText"`
}

func (t textRuns) String() string {
	if t.SimpleText != "" {
		return t.SimpleText
	}
	var b strings.Builder
	for _, r := range t.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

type thumbs struct {
	Thumbnails []struct {
		URL string `json:"url"`
	} `json:"thumbnails"`
}

func (t thumbs) last() string {
	if len(t.Thumbnails) == 0 {
		return ""
	}
	u := t.Thumbnails[len(t.Thumbnails)-1].URL
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	return u
}

type airing struct {
	title, network, rating, slot, art, mark string
	ends                                    time.Time
	ctt                                     string
}

func unpluggedNext(ctx context.Context, hc *http.Client, ctt, id string) (airing, error) {
	body := map[string]any{
		"videoId": id,
		"context": map[string]any{
			"client": map[string]string{"hl": "en", "gl": "US", "clientName": licenseClient, "clientVersion": licenseVersion},
			"user":   credential(ctt),
		},
	}
	var a struct {
		Contents struct {
			Single struct {
				Results struct {
					Tabbed struct {
						Metadata struct {
							Renderer struct {
								PrimaryText   textRuns `json:"primaryText"`
								SecondaryText textRuns `json:"secondaryText"`
								TertiaryText  textRuns `json:"tertiaryText"`
								NetworkName   textRuns `json:"networkName"`
								EndTime       string   `json:"endTimeSeconds"`
								Thumbnail     thumbs   `json:"thumbnail"`
								NetworkIcon   thumbs   `json:"networkIcon"`
							} `json:"unpluggedVideoMetadataRenderer"`
						} `json:"videoMetadata"`
					} `json:"watchNextTabbedResultsRenderer"`
				} `json:"results"`
			} `json:"singleColumnWatchNextResults"`
		} `json:"contents"`
		Current struct {
			Watch struct {
				Config struct {
					Token struct {
						Transfer []struct {
							Token string `json:"token"`
						} `json:"credentialTransferTokens"`
					} `json:"videoAuthorizationToken"`
				} `json:"watchEndpointSupportedAuthorizationTokenConfig"`
			} `json:"watchEndpoint"`
		} `json:"currentVideoEndpoint"`
	}
	err := postJSON(ctx, hc, guideURL, body, map[string]string{"Origin": licenseOrigin, "X-Origin": licenseOrigin}, &a)
	if err != nil {
		return airing{}, fmt.Errorf("youtube tv: next: %w", err)
	}
	m := a.Contents.Single.Results.Tabbed.Metadata.Renderer
	out := airing{
		title:   m.PrimaryText.String(),
		network: m.NetworkName.String(),
		rating:  m.SecondaryText.String(),
		slot:    strings.TrimSpace(strings.TrimPrefix(m.TertiaryText.String(), "Live •")),
	}
	out.art, out.mark = m.Thumbnail.last(), m.NetworkIcon.last()
	if s, err := strconv.ParseInt(m.EndTime, 10, 64); err == nil && s > 0 {
		out.ends = time.Unix(s, 0)
	}
	if t := a.Current.Watch.Config.Token.Transfer; len(t) > 0 {
		out.ctt = t[0].Token
	}
	if out.title == "" {
		return out, errors.New("youtube tv: next carried no airing")
	}
	return out, nil
}

func (u *unplugged) guide(ctx context.Context) {
	for ctx.Err() == nil {
		u.licMu.Lock()
		ctt := u.ctt
		u.licMu.Unlock()
		a, err := unpluggedNext(ctx, u.hc, ctt, u.id)
		next := airingRetry
		if err != nil {
			slog.Warn("youtube tv guide", "video", u.id, "err", err)
		} else {
			if a.ctt != "" {
				u.licMu.Lock()
				u.ctt = a.ctt
				u.licMu.Unlock()
			}
			u.mu.Lock()
			u.info.Title = a.title
			if a.network != "" {
				u.info.Author = a.network
			}
			u.album = strings.Join(nonEmpty(a.rating, a.slot), " · ")
			if a.art != "" {
				u.info.Thumbnail = a.art
			}
			if a.mark != "" {
				u.mark = a.mark
			}
			changed := u.changed
			u.mu.Unlock()
			slog.Info("youtube tv airing", "video", u.id, "title", a.title, "network", a.network, "slot", a.slot, "ends", a.ends)
			if changed != nil {
				changed()
			}
			if !a.ends.IsZero() {
				next = max(time.Until(a.ends)+airingSlack, airingSlack)
			}
		}
		wait(ctx, next)
	}
}

const heartbeatEvery = 30 * time.Second

var heartbeatChecks = []string{"HEARTBEAT_CHECK_TYPE_LIVE_STREAM_STATUS", "HEARTBEAT_CHECK_TYPE_YPC", "HEARTBEAT_CHECK_TYPE_UNPLUGGED"}

func (u *unplugged) heartbeat(ctx context.Context, token, data, interval string) {
	every := heartbeatEvery
	if ms, err := strconv.Atoi(interval); err == nil && ms > 0 {
		every = time.Duration(ms) * time.Millisecond
	}
	for seq := 0; ctx.Err() == nil; seq++ {
		wait(ctx, every)
		if ctx.Err() != nil {
			return
		}
		u.licMu.Lock()
		ctt := u.ctt
		u.licMu.Unlock()
		at := strconv.FormatInt(time.Now().Add(-liveLead()).UnixMilli(), 10)
		body := map[string]any{
			"videoId":             u.id,
			"sequenceNumber":      seq,
			"heartbeatServerData": data,
			"heartbeatToken":      token,
			"cpn":                 u.cpn,
			"heartbeatRequestParams": map[string]any{
				"heartbeatChecks": heartbeatChecks,
				"unpluggedParams": map[string]string{"clientPlayerPositionUtcMillis": at},
			},
			"playbackState": map[string]any{"playbackPosition": map[string]string{"utcTimeMillis": at}},
			"context": map[string]any{
				"client": map[string]string{"hl": "en", "gl": "US", "clientName": licenseClient, "clientVersion": licenseVersion},
				"user":   credential(ctt),
			},
		}
		var a struct {
			Playability struct {
				Status string `json:"status"`
				Reason string `json:"reason"`
			} `json:"playabilityStatus"`
			Poll       string `json:"pollDelayMs"`
			ServerData string `json:"heartbeatServerData"`
		}
		err := postJSON(ctx, u.hc, heartbeatURL, body, map[string]string{
			"Origin": licenseOrigin, "X-Origin": licenseOrigin,
			"X-Youtube-Client-Name": licenseNumber, "X-Youtube-Client-Version": licenseVersion,
		}, &a)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("youtube tv heartbeat", "video", u.id, "seq", seq, "err", err)
			}
			continue
		}
		if a.Playability.Status != "OK" {
			slog.Warn("youtube tv heartbeat refused", "video", u.id, "status", a.Playability.Status, "reason", a.Playability.Reason)
			u.fail(fmt.Errorf("youtube tv: %s", strings.ToLower(strings.Join(nonEmpty(a.Playability.Status, a.Playability.Reason), ": "))))
			return
		}
		if a.ServerData != "" {
			data = a.ServerData
		}
		if ms, err := strconv.Atoi(a.Poll); err == nil && ms > 0 {
			every = time.Duration(ms) * time.Millisecond
		}
		if seq == 0 {
			slog.Info("youtube tv heartbeat", "video", u.id, "every", every)
		}
	}
}

func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (u *unplugged) showing() (Track, string, string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.info, u.album, u.mark
}

func (u *unplugged) onChange(f func()) {
	u.mu.Lock()
	u.changed = f
	u.mu.Unlock()
}

type sealedFrame struct {
	data  []byte
	at    time.Duration
	crypt *surface.Crypt
}

type unplugged struct {
	hc       *http.Client
	surf     *surface.Client
	resample func(from int) func([]int16) []int16
	ctt, id  string
	cpn      string
	session  string
	params   string
	drm      uint32
	audioID  uint32

	Width, Height int
	info          Track
	album         string
	mark          string
	changed       func()

	cancel    context.CancelFunc
	done      chan struct{}
	once      sync.Once
	ready     chan struct{}
	readyOnce sync.Once

	mu       sync.Mutex
	room     *sync.Cond
	pcm      []int16
	err      error
	origin   time.Duration
	based    bool
	heard    bool
	licensed map[string]bool
	licMu    sync.Mutex
	resamp   func([]int16) []int16
	rate     int

	frames chan sealedFrame
}

func openUnplugged(ctx context.Context, env surfaceEnv, id, params, ctt string, at time.Duration) (*unplugged, error) {
	if ctt == "" {
		return nil, errors.New("youtube tv: the phone sent no credential")
	}
	surf := env.surface()
	if surf == nil {
		return nil, errors.New("youtube tv: the display helper is not connected")
	}
	a, err := unpluggedPlay(ctx, env.http, ctt, id, params)
	if err != nil {
		return nil, err
	}
	video, audio := pickUnplugged(a.StreamingData.AdaptiveFormats)
	if video == nil || audio == nil {
		return nil, errors.New("youtube tv: no H.264 and AAC formats offered")
	}
	cert, err := base64.RawURLEncoding.DecodeString(widevineCert)
	if err != nil {
		return nil, err
	}
	drm := 1 + drmIDs.Add(1)
	if err := surf.DRMOpen(drm, surface.Widevine, true, cert); err != nil {
		return nil, fmt.Errorf("youtube tv: widevine: %w", err)
	}
	run, cancel := context.WithCancel(context.Background())
	u := &unplugged{
		hc: env.http, surf: surf, resample: env.resample, ctt: ctt, id: id, cpn: nonce(),
		session: a.HeartbeatParams.DRMSessionID, params: a.StreamingData.DRMParams,
		drm: drm, audioID: 1 + audioIDs.Add(1),
		Width: video.Width, Height: video.Height,
		cancel: cancel, done: make(chan struct{}), ready: make(chan struct{}),
		licensed: map[string]bool{}, frames: make(chan sealedFrame, 300),
	}
	u.room = sync.NewCond(&u.mu)
	u.info = Track{ID: id, Title: a.VideoDetails.Title, Author: a.VideoDetails.Author, Codec: "aac"}
	if th := a.VideoDetails.Thumbnail.Thumbnails; len(th) > 0 {
		u.info.Thumbnail = th[len(th)-1].URL
		if strings.HasPrefix(u.info.Thumbnail, "//") {
			u.info.Thumbnail = "https:" + u.info.Thumbnail
		}
	}
	slog.Info("youtube tv formats", "video", id, "picture", fmt.Sprintf("%dx%d@%d itag %d", video.Width, video.Height, video.FPS, video.Itag), "audio itag", audio.Itag)

	head, err := u.head(run, audio.URL, a.PlayerConfig.Live.MinDvrSequence)
	if err != nil {
		u.shut()
		return nil, err
	}
	oldest, _ := strconv.ParseInt(a.PlayerConfig.Live.MinDvrSequence, 10, 64)
	start := tvStart(head, oldest, at, time.Now())
	go func() { u.fail(u.follow(run, video.URL, start, true)) }()
	go func() { u.fail(u.follow(run, audio.URL, start, false)) }()
	go u.guide(run)
	go u.heartbeat(run, a.HeartbeatParams.Token, a.HeartbeatParams.ServerData, a.HeartbeatParams.Interval)

	wait := time.NewTimer(tvStartWait)
	defer wait.Stop()
	select {
	case <-u.ready:
		return u, nil
	case <-u.done:
		err := u.failure()
		u.shut()
		return nil, err
	case <-ctx.Done():
		u.shut()
		return nil, ctx.Err()
	case <-wait.C:
		u.shut()
		return nil, errors.New("youtube tv: no picture arrived")
	}
}

func (u *unplugged) head(ctx context.Context, base, from string) (int64, error) {
	sq, _ := strconv.ParseInt(from, 10, 64)
	_, hdr, err := u.segment(ctx, base, sq)
	if err != nil {
		return 0, err
	}
	head, err := strconv.ParseInt(hdr.Get("X-Head-Seqnum"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("youtube tv: no live edge in %q", hdr.Get("X-Head-Seqnum"))
	}
	return head, nil
}

func (u *unplugged) segment(ctx context.Context, base string, sq int64) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"&sq="+strconv.FormatInt(sq, 10), nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", unpluggedAgent)
	resp, err := u.hc.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, resp.Header, &statusError{code: resp.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, segmentLimit))
	return b, resp.Header, err
}

type statusError struct{ code int }

func (e *statusError) Error() string { return "youtube tv: segment " + strconv.Itoa(e.code) }

func (u *unplugged) follow(ctx context.Context, base string, sq int64, video bool) error {
	var track *cenc.Track
	misses := 0
	for ctx.Err() == nil {
		b, hdr, err := u.segment(ctx, base, sq)
		if err != nil {
			var se *statusError
			if errors.As(err, &se) && misses < 30 {
				if head, _ := strconv.ParseInt(hdr.Get("X-Head-Seqnum"), 10, 64); head > 0 && sq > head {
					misses++
					wait(ctx, segmentWait)
					continue
				}
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			misses++
			if misses > 30 {
				return fmt.Errorf("youtube tv: sq %d: %w", sq, err)
			}
			wait(ctx, segmentWait)
			continue
		}
		misses = 0
		t, err := cenc.ParseInit(b)
		if err != nil {
			return fmt.Errorf("youtube tv: sq %d: %w", sq, err)
		}
		first := track == nil
		if first {
			track = &t
		}
		samples, pssh, err := cenc.ParseFragment(*track, b)
		if err != nil {
			return fmt.Errorf("youtube tv: sq %d: %w", sq, err)
		}
		if err := u.license(ctx, append(track.PSSH, pssh...)); err != nil {
			return err
		}
		if first && !video {
			if err := u.surf.AudioOpen(u.audioID, u.drm, t.Rate, t.Channels, t.Config); err != nil {
				return fmt.Errorf("youtube tv: audio decoder: %w", err)
			}
			u.mu.Lock()
			u.rate = t.Rate
			u.mu.Unlock()
		}
		for _, s := range samples {
			if video {
				if err := u.picture(ctx, *track, s); err != nil {
					return err
				}
			} else if err := u.sound(s); err != nil {
				return err
			}
		}
		sq++
	}
	return ctx.Err()
}

func widevinePSSH(boxes [][]byte) []byte {
	for _, b := range boxes {
		if len(b) >= 28 && bytes.Equal(b[12:28], surface.Widevine[:]) {
			return b
		}
	}
	return nil
}

func (u *unplugged) license(ctx context.Context, boxes [][]byte) error {
	init := widevinePSSH(boxes)
	if init == nil {
		return nil
	}
	key := string(init)
	u.licMu.Lock()
	defer u.licMu.Unlock()
	if u.licensed[key] {
		return nil
	}
	challenge, err := u.surf.DRMRequest(u.drm, init)
	if err != nil {
		return fmt.Errorf("youtube tv: key request: %w", err)
	}
	lic, err := unpluggedLicense(ctx, u.hc, u.ctt, u.id, u.cpn, u.session, u.params, challenge)
	if err != nil {
		return err
	}
	if err := u.surf.DRMProvide(u.drm, lic); err != nil {
		return fmt.Errorf("youtube tv: key response: %w", err)
	}
	u.licensed[key] = true
	slog.Info("youtube tv licensed", "video", u.id, "keys", len(u.licensed))
	return nil
}

func crypt(s cenc.Sample, subs []cenc.Subsample) *surface.Crypt {
	if !s.Encrypted {
		return nil
	}
	k := &surface.Crypt{Mode: surface.CryptCENC, Key: s.KeyID, IV: s.IV}
	for _, x := range subs {
		k.Subsamples = append(k.Subsamples, surface.Subsample{Clear: x.Clear, Encrypted: x.Encrypted})
	}
	return k
}

func (u *unplugged) anchor(at time.Duration) time.Duration {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.based {
		u.origin, u.based = at, true
		u.room.Broadcast()
	}
	return at - u.origin
}

func (u *unplugged) picture(ctx context.Context, t cenc.Track, s cenc.Sample) error {
	data, subs, err := t.AnnexB(s)
	if err != nil {
		return err
	}
	at := u.anchor(s.At)
	if at < 0 {
		return nil
	}
	select {
	case u.frames <- sealedFrame{data: data, at: at, crypt: crypt(s, subs)}:
		u.readyOnce.Do(func() { close(u.ready) })
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (u *unplugged) sound(s cenc.Sample) error {
	u.mu.Lock()
	for !u.based {
		select {
		case <-u.done:
			u.mu.Unlock()
			return io.EOF
		default:
		}
		u.room.Wait()
	}
	origin, first := u.origin, !u.heard
	u.mu.Unlock()
	if s.At < origin {
		return nil
	}
	if first {
		u.mu.Lock()
		u.heard = true
		gap := int(int64(s.At-origin) * rate / int64(time.Second))
		u.pcm = append(u.pcm, make([]int16, gap*channels)...)
		u.mu.Unlock()
	}
	k := crypt(s, s.Subsamples)
	for {
		out, err := u.surf.AudioSample(u.audioID, s.At-origin, 0, k, s.Data)
		if len(out.Data) > 0 {
			if err := u.keep(out); err != nil {
				return err
			}
		}
		if !errors.Is(err, surface.Full) {
			if err != nil {
				return fmt.Errorf("youtube tv: audio: %w", err)
			}
			return nil
		}
		select {
		case <-u.done:
			return io.EOF
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (u *unplugged) keep(out surface.PCM) error {
	pcm := make([]int16, len(out.Data)/2)
	for i := range pcm {
		pcm[i] = int16(binary.LittleEndian.Uint16(out.Data[2*i:]))
	}
	if out.Channels == 1 {
		st := make([]int16, 2*len(pcm))
		for i, v := range pcm {
			st[2*i], st[2*i+1] = v, v
		}
		pcm = st
	}
	u.mu.Lock()
	if u.resamp == nil || out.Rate != u.rate {
		u.rate = out.Rate
		if out.Rate != rate && u.resample != nil {
			u.resamp = u.resample(out.Rate)
		} else {
			u.resamp = func(x []int16) []int16 { return x }
		}
	}
	pcm = u.resamp(pcm)
	for len(u.pcm) > heldSeconds*rate*channels {
		u.room.Wait()
		select {
		case <-u.done:
			u.mu.Unlock()
			return io.EOF
		default:
		}
	}
	u.pcm = append(u.pcm, pcm...)
	u.mu.Unlock()
	return nil
}

func (u *unplugged) Read(out []int16) (int, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	n := copy(out, u.pcm)
	u.pcm = u.pcm[n:]
	if n > 0 {
		u.room.Broadcast()
		return n, nil
	}
	select {
	case <-u.done:
		return 0, u.err
	default:
		return 0, nil
	}
}

func (u *unplugged) next() ([]byte, time.Duration, *surface.Crypt, error) {
	select {
	case f := <-u.frames:
		return f.data, f.at, f.crypt, nil
	case <-u.done:
		return nil, 0, nil, u.failure()
	}
}

func (u *unplugged) Picture() playback.Picture {
	return playback.Picture{H264: true, Width: u.Width, Height: u.Height, Session: u.drm, Sealed: u.next}
}

func (u *unplugged) fail(err error) {
	u.mu.Lock()
	if u.err == nil {
		u.err = err
	}
	u.mu.Unlock()
	u.stop()
}

func (u *unplugged) stop() {
	u.once.Do(func() {
		u.cancel()
		close(u.done)
		u.mu.Lock()
		u.room.Broadcast()
		u.mu.Unlock()
	})
}

func (u *unplugged) failure() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.err == nil {
		return io.EOF
	}
	return u.err
}

func (u *unplugged) shut() {
	u.fail(io.EOF)
	u.surf.AudioClose(u.audioID)
	u.surf.DRMClose(u.drm)
}

func (u *unplugged) Close() { u.shut() }

const tvSegment = 5 * time.Second

func tvStart(head, oldest int64, at time.Duration, now time.Time) int64 {
	lead := max(int64(liveLead()/tvSegment), 1)
	start := head - lead
	if wallClock(at) {
		if back := int64(now.Sub(time.Unix(0, 0).Add(at)) / tvSegment); back > lead {
			start = max(head-back, oldest)
		}
	}
	return start
}

func liveLead() time.Duration {
	return time.Duration(config.Get().Cast.YouTube.LiveDelay) * time.Second
}

type surfaceEnv struct {
	http     *http.Client
	surface  func() *surface.Client
	resample func(from int) func([]int16) []int16
}
