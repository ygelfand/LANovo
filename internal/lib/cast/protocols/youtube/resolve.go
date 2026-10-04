package youtube

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	yt "github.com/kkdai/youtube/v2"

	"github.com/ygelfand/LANovo/internal/lib/cast/playback"
	"github.com/ygelfand/LANovo/internal/lib/fetch"
	"github.com/ygelfand/LANovo/internal/lib/hls"
)

var visionOS = yt.ClientInfo{
	Name:        "VISIONOS",
	Version:     "1.02",
	UserAgent:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_7_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15",
	DeviceModel: "RealityDevice17,1",
}

var useClient sync.Once

// Track is a video as the player needs it.
type Track struct {
	ID        string
	Title     string
	Author    string
	Duration  time.Duration
	Thumbnail string

	// Codec is the stream's, "opus" for WebM Opus.
	Codec string
}

const (
	chunkTime  = 15 * time.Second
	chunkFloor = 64 << 10
	ahead      = 2
	lead       = 30 * time.Second
	retryAfter = 5 * time.Second
)

func rateOf(f *yt.Format) int64 {
	ms, err := strconv.ParseInt(f.ApproxDurationMs, 10, 64)
	if err != nil || ms <= 0 || f.ContentLength <= 0 {
		return 0
	}
	return f.ContentLength * 1000 / ms
}

func chunkFor(f *yt.Format) int64 {
	return max(rateOf(f)*int64(chunkTime/time.Second), chunkFloor)
}

// Resolver turns video ids into tracks and their audio.
type Resolver struct {
	client   yt.Client
	http     *http.Client
	Resample func(from int) func(stereo []int16) []int16
	target   func() playback.Target
}

type Live struct {
	Track Track
	HLS   string
}

func (l *Live) Error() string { return fmt.Sprintf("youtube: %s is a live stream", l.Track.ID) }

func NewResolver(c *http.Client, target func() playback.Target) *Resolver {
	useClient.Do(func() { yt.DefaultClient = visionOS })
	return &Resolver{client: yt.Client{HTTPClient: c}, http: c, target: target}
}

// Info is a video's title, author, length and thumbnail, without opening anything.
func (r *Resolver) Info(ctx context.Context, id string) (Track, error) {
	v, err := r.client.GetVideoContext(ctx, id)
	if err != nil {
		return Track{}, fmt.Errorf("youtube: resolving %s: %w", id, err)
	}
	t := Track{ID: id, Title: v.Title, Author: v.Author, Duration: v.Duration}
	if len(v.Thumbnails) > 0 {
		t.Thumbnail = v.Thumbnails[len(v.Thumbnails)-1].URL
	}
	return t, nil
}

func (r *Resolver) Formats(ctx context.Context, id string) (string, error) {
	v, err := r.client.GetVideoContext(ctx, id)
	if err != nil {
		return "", fmt.Errorf("youtube: resolving %s: %w", id, err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "title     %q\n", v.Title)
	fmt.Fprintf(&b, "author    %q\n", v.Author)
	fmt.Fprintf(&b, "duration  %v\n", v.Duration)
	fmt.Fprintf(&b, "live      %v\n", live(v))
	fmt.Fprintf(&b, "hls       %v\n", v.HLSManifestURL != "")
	chosen := func(f *yt.Format) string {
		if f == nil {
			return "none"
		}
		return strconv.Itoa(f.ItagNo)
	}
	fmt.Fprintf(&b, "audio     %s\n", chosen(bestOpus(v.Formats)))
	fmt.Fprintf(&b, "video     %s\n", chosen(bestVP9(v.Formats, r.limits())))
	for _, f := range v.Formats {
		track := ""
		if f.AudioTrack != nil {
			track = f.AudioTrack.ID
		}
		fmt.Fprintf(&b, "itag %-4d %-36s %4dp %3dfps %8d bps %10d bytes %s\n",
			f.ItagNo, f.MimeType, f.Height, f.FPS, f.Bitrate, f.ContentLength, track)
	}
	if live(v) {
		d, err := hls.Describe(ctx, v.HLSManifestURL, hls.Options{HTTP: r.http, Ask: ask, Tallest: r.limits().Tallest, Fastest: r.limits().Fastest})
		if err != nil {
			fmt.Fprintf(&b, "hls: %v\n", err)
		}
		b.WriteString(d)
	}
	return b.String(), nil
}

func live(v *yt.Video) bool { return v.HLSManifestURL != "" && v.Duration == 0 }

// Audio resolves a video and opens its best Opus stream.
func (r *Resolver) Audio(ctx context.Context, id string) (Track, io.ReadCloser, error) {
	t, stream, err := r.open(ctx, id, "opus audio", bestOpus, ahead)
	t.Codec = "opus"
	return t, stream, err
}

// Video opens a video's best VP9 picture stream, which carries no sound.
func (r *Resolver) Video(ctx context.Context, id string) (Track, io.ReadCloser, error) {
	t, stream, err := r.open(ctx, id, "vp9 video", func(f yt.FormatList) *yt.Format { return bestVP9(f, r.limits()) }, 4)
	t.Codec = "vp9"
	return t, stream, err
}

func (r *Resolver) open(ctx context.Context, id, what string, pick func(yt.FormatList) *yt.Format, ahead int) (Track, io.ReadCloser, error) {
	v, err := r.client.GetVideoContext(ctx, id)
	if err != nil {
		return Track{}, nil, fmt.Errorf("youtube: resolving %s: %w", id, err)
	}
	if live(v) {
		return Track{}, nil, &Live{Track: trackOf(id, v), HLS: v.HLSManifestURL}
	}

	f := pick(v.Formats)
	if f == nil {
		offered := make([]string, 0, len(v.Formats))
		for _, c := range v.Formats {
			offered = append(offered, fmt.Sprintf("%d:%s", c.ItagNo, c.MimeType))
		}
		slog.Warn("youtube formats without a pick", "video", id, "want", what, "duration", v.Duration, "hls", v.HLSManifestURL != "", "offered", strings.Join(offered, " "))
		return Track{}, nil, fmt.Errorf("youtube: %s has no %s", id, what)
	}
	for i := range v.Formats {
		c := &v.Formats[i]
		if c.ItagNo != f.ItagNo {
			continue
		}
		track := ""
		if c.AudioTrack != nil {
			track = c.AudioTrack.ID
		}
		slog.Debug("youtube format", "video", id, "what", what, "chosen", c == f, "itag", c.ItagNo, "mime", c.MimeType,
			"bitrate", c.Bitrate, "length", c.ContentLength, "ms", c.ApproxDurationMs, "track", track,
			"init", c.InitRange, "index", c.IndexRange)
	}
	slog.Debug("youtube stream", "video", id, "what", what, "itag", f.ItagNo, "chunk", chunkFor(f), "rate", rateOf(f), "lead", lead, "ahead", ahead)
	url, err := r.client.GetStreamURLContext(ctx, v, f)
	if err != nil {
		return Track{}, nil, fmt.Errorf("youtube: opening %s: %w", id, err)
	}
	renew := func(ctx context.Context) (string, error) {
		fresh, err := r.client.GetVideoContext(ctx, id)
		if err != nil {
			return "", err
		}
		if f := pick(fresh.Formats); f != nil {
			return r.client.GetStreamURLContext(ctx, fresh, f)
		}
		return "", fmt.Errorf("youtube: %s has no %s", id, what)
	}
	stream := fetch.NewRanged(ctx, r.http, url, f.ContentLength, fetch.Chunks{Size: chunkFor(f), Rate: rateOf(f), Lead: lead, Retry: retryAfter, Ahead: ahead, Ask: ask, Renew: renew})

	return trackOf(id, v), stream, nil
}

func trackOf(id string, v *yt.Video) Track {
	t := Track{ID: id, Title: v.Title, Author: v.Author, Duration: v.Duration}
	if len(v.Thumbnails) > 0 {
		t.Thumbnail = v.Thumbnails[len(v.Thumbnails)-1].URL
	}
	return t
}

func ask(req *http.Request) {
	req.Header.Set("User-Agent", visionOS.UserAgent)
	req.Header.Set("Origin", "https://youtube.com")
}

func (r *Resolver) limits() playback.Target {
	if r.target == nil {
		return playback.Target{}
	}
	return r.target()
}

func bestVP9(formats yt.FormatList, limits playback.Target) *yt.Format {
	var found []*yt.Format
	for i := range formats {
		f := &formats[i]
		if strings.HasPrefix(f.MimeType, "video/webm") && strings.Contains(f.MimeType, "vp9") && within(f.Height, limits.Tallest) {
			found = append(found, f)
		}
	}
	if len(found) == 0 {
		return nil
	}
	sort.Slice(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if within(a.FPS, limits.Fastest) != within(b.FPS, limits.Fastest) {
			return within(a.FPS, limits.Fastest)
		}
		if a.Height != b.Height {
			return a.Height > b.Height
		}
		return a.Bitrate > b.Bitrate
	})
	return found[0]
}

// bestOpus is the highest-bitrate audio-only Opus format.
func bestOpus(formats yt.FormatList) *yt.Format {
	var found []*yt.Format
	for i := range formats {
		f := &formats[i]
		if strings.HasPrefix(f.MimeType, "audio/webm") && strings.Contains(f.MimeType, "opus") {
			found = append(found, f)
		}
	}
	if len(found) == 0 {
		return nil
	}
	sort.SliceStable(found, func(i, j int) bool {
		a, b := audioOf(found[i]), audioOf(found[j])
		if a.plain != b.plain {
			return a.plain
		}
		if a.original != b.original {
			return a.original
		}
		return found[i].Bitrate > found[j].Bitrate
	})
	return found[0]
}

type audio struct{ plain, original bool }

// audioOf reads which rendition an audio format is: the original language or a dub, and whether it
// has been compressed or had voices lifted.
func audioOf(f *yt.Format) audio {
	tags := xtags(f.URL)
	a := audio{plain: tags["drc"] == "" && tags["vb"] == ""}
	switch {
	case tags["acont"] != "":
		a.original = tags["acont"] == "original"
	case f.AudioTrack != nil:
		a.original = f.AudioTrack.AudioIsDefault
	default:
		a.original = true
	}
	return a
}

func xtags(raw string) map[string]string {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	tags := map[string]string{}
	for _, pair := range strings.Split(u.Query().Get("xtags"), ":") {
		if k, v, ok := strings.Cut(pair, "="); ok {
			tags[k] = v
		}
	}
	return tags
}
