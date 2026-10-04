package youtube

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/lib/cast/playback"
)

const SponsorBlockURL = "https://sponsor.ajay.app/api/skipSegments"

const sponsorBlockWait = 3 * time.Second

var Categories = []string{"sponsor", "selfpromo", "interaction", "intro", "outro", "preview", "hook", "music_offtopic", "filler"}

func ParseCategories(text string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
		if slices.Contains(Categories, w) && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

type Segment struct {
	From, To time.Duration
	Category string
}

var colors = map[string]uint32{
	"sponsor":        0x00d400,
	"selfpromo":      0xffff00,
	"interaction":    0xcc00ff,
	"intro":          0x00ffff,
	"outro":          0x0202ed,
	"preview":        0x008fd6,
	"hook":           0x395699,
	"music_offtopic": 0xff9900,
	"filler":         0x7300ff,
}

func marks(segs []Segment) []playback.Mark {
	out := make([]playback.Mark, 0, len(segs))
	for _, s := range segs {
		out = append(out, playback.Mark{From: s.From, To: s.To, RGB: colors[s.Category]})
	}
	return out
}

func Segments(ctx context.Context, c *http.Client, endpoint, id string, categories []string) ([]Segment, error) {
	if len(categories) == 0 {
		return nil, nil
	}

	sum := sha256.Sum256([]byte(id))
	q := url.Values{"category": categories, "actionType": {"skip"}, "service": {"YouTube"}}
	at := endpoint + "/" + hex.EncodeToString(sum[:2]) + "?" + q.Encode()

	ctx, cancel := context.WithTimeout(ctx, sponsorBlockWait)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, at, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sponsorblock: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sponsorblock: %s", resp.Status)
	}

	var videos []struct {
		VideoID  string `json:"videoID"`
		Segments []struct {
			Category   string    `json:"category"`
			ActionType string    `json:"actionType"`
			Segment    []float64 `json:"segment"`
		} `json:"segments"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&videos); err != nil {
		return nil, fmt.Errorf("sponsorblock: %w", err)
	}

	var out []Segment
	for _, v := range videos {
		if v.VideoID != id {
			continue
		}
		for _, s := range v.Segments {
			if s.ActionType != "skip" || !slices.Contains(categories, s.Category) || len(s.Segment) != 2 {
				continue
			}
			from, to := s.Segment[0], s.Segment[1]
			if !finite(from) || !finite(to) || from < 0 || to <= from {
				continue
			}
			out = append(out, Segment{From: seconds(from), To: seconds(to), Category: s.Category})
		}
	}
	return merged(out), nil
}

func (s *station) segments(ctx context.Context, r *Resolver, id string) []Segment {
	if s == nil || s.theme != ThemeYouTube {
		return nil
	}
	s.mu.Lock()
	if s.skipsFor == id {
		defer s.mu.Unlock()
		return s.skips
	}
	s.mu.Unlock()

	segs, err := Segments(ctx, r.http, SponsorBlockURL, id, config.Get().Cast.YouTube.Skip)
	if err != nil {
		slog.Warn("youtube sponsorblock", "video", id, "err", err)
		return nil
	}
	if len(segs) > 0 {
		slog.Debug("youtube sponsorblock", "video", id, "segments", len(segs))
	}
	s.mu.Lock()
	s.skipsFor, s.skips = id, segs
	s.mu.Unlock()
	return segs
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

func merged(in []Segment) []Segment {
	slices.SortFunc(in, func(a, b Segment) int { return int(a.From - b.From) })

	var out []Segment
	for _, s := range in {
		if n := len(out); n > 0 && s.From <= out[n-1].To {
			out[n-1].To = max(out[n-1].To, s.To)
			continue
		}
		out = append(out, s)
	}
	return out
}

func inside(segments []Segment, start, t time.Duration) (Segment, bool) {
	for _, s := range segments {
		if s.From >= start && t >= s.From && t < s.To {
			return s, true
		}
	}
	return Segment{}, false
}
