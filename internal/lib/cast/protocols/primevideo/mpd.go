package primevideo

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/cenc"
)

type mpd struct {
	Base     string   `xml:"BaseURL"`
	Type     string   `xml:"type,attr"`
	Duration string   `xml:"mediaPresentationDuration,attr"`
	Periods  []period `xml:"Period"`
}

type period struct {
	ID       string       `xml:"id,attr"`
	Start    string       `xml:"start,attr"`
	Duration string       `xml:"duration,attr"`
	Base     string       `xml:"BaseURL"`
	Sets     []adaptation `xml:"AdaptationSet"`
}

type adaptation struct {
	Content string `xml:"contentType,attr"`
	Mime    string `xml:"mimeType,attr"`
	Codecs  string `xml:"codecs,attr"`
	Lang    string `xml:"lang,attr"`
	Base    string `xml:"BaseURL"`
	Roles   []struct {
		Value string `xml:"value,attr"`
	} `xml:"Role"`
	Protection []protection     `xml:"ContentProtection"`
	Segment    *segmentBase     `xml:"SegmentBase"`
	List       *segmentList     `xml:"SegmentList"`
	Reps       []representation `xml:"Representation"`
}

type segmentList struct {
	Duration  int64 `xml:"duration,attr"`
	Timescale int64 `xml:"timescale,attr"`
	Offset    int64 `xml:"presentationTimeOffset,attr"`
	Init      *struct {
		Range string `xml:"range,attr"`
	} `xml:"Initialization"`
	URLs []struct {
		Range string `xml:"mediaRange,attr"`
	} `xml:"SegmentURL"`
	Timeline []struct {
		T int64 `xml:"t,attr"`
		D int64 `xml:"d,attr"`
		R int64 `xml:"r,attr"`
	} `xml:"SegmentTimeline>S"`
}

type segmentBase struct {
	Timescale int64  `xml:"timescale,attr"`
	Offset    int64  `xml:"presentationTimeOffset,attr"`
	Index     string `xml:"indexRange,attr"`
	Init      *struct {
		Source string `xml:"sourceURL,attr"`
		Range  string `xml:"range,attr"`
	} `xml:"Initialization"`
	Represented *struct {
		Source string `xml:"sourceURL,attr"`
		Range  string `xml:"range,attr"`
	} `xml:"RepresentationIndex"`
}

type protection struct {
	Scheme string `xml:"schemeIdUri,attr"`
	KID    string `xml:"default_KID,attr"`
}

type representation struct {
	ID        string       `xml:"id,attr"`
	Bandwidth int          `xml:"bandwidth,attr"`
	Width     int          `xml:"width,attr"`
	Height    int          `xml:"height,attr"`
	Codecs    string       `xml:"codecs,attr"`
	Mime      string       `xml:"mimeType,attr"`
	Base      string       `xml:"BaseURL"`
	Segment   *segmentBase `xml:"SegmentBase"`
	List      *segmentList `xml:"SegmentList"`
}

func Summarize(data []byte) (string, error) {
	var m mpd
	if err := xml.Unmarshal(data, &m); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "type=%s duration=%s periods=%d", m.Type, m.Duration, len(m.Periods))
	for _, p := range m.Periods {
		fmt.Fprintf(&b, " | period %s %s:", p.ID, p.Duration)
		for _, a := range p.Sets {
			kind := a.Content
			if kind == "" {
				kind = a.Mime
			}
			var schemes []string
			for _, c := range a.Protection {
				schemes = append(schemes, strings.TrimPrefix(c.Scheme, "urn:uuid:"))
			}
			lo, hi := "", ""
			for i, r := range a.Reps {
				d := fmt.Sprintf("%dx%d@%dk %s", r.Width, r.Height, r.Bandwidth/1000, first(r.Codecs, a.Codecs))
				if i == 0 {
					lo = d
				}
				hi = d
			}
			base := ""
			if len(a.Reps) > 0 {
				r := a.Reps[0]
				base = "base=" + r.Base
				if r.Segment != nil {
					base += " index=" + r.Segment.Index
				}
			}
			fmt.Fprintf(&b, " [%s %s reps=%d %s..%s drm=%s %s]", kind, a.Lang, len(a.Reps), lo, hi, strings.Join(schemes, ","), base)
		}
	}
	return b.String(), nil
}

func first(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

type Rep struct {
	URL           string
	InitURL       string
	IndexURL      string
	Init, Index   [2]int64
	Frags         []cenc.Fragment
	Offset        time.Duration
	KID           string
	Width, Height int
	Bandwidth     int
	Codecs        string
	Lang          string
}

type Part struct {
	ID              string
	Start, Duration time.Duration
	Video, Audio    Rep
}

func Pick(data []byte, manifest string, tallest int, lang string) ([]Part, error) {
	var m mpd
	if err := xml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if len(m.Periods) == 0 {
		return nil, fmt.Errorf("prime video: manifest has no period")
	}
	root, err := url.Parse(manifest)
	if err != nil {
		return nil, err
	}
	want := strings.ToLower(strings.SplitN(lang, "-", 2)[0])
	var parts []Part
	var at time.Duration
	for _, p := range m.Periods {
		if p.Start != "" {
			at = ParseDuration(p.Start)
		}
		video, audio, ok := pickPeriod(p, resolve(root, m.Base, p.Base), tallest, want)
		if !ok {
			return nil, fmt.Errorf("prime video: no H.264 video and AAC audio under %dp in period %q: %s", tallest, p.ID, excerpt(data))
		}
		part := Part{ID: p.ID, Start: at, Duration: ParseDuration(p.Duration), Video: video, Audio: audio}
		parts = append(parts, part)
		at += part.Duration
	}
	return parts, nil
}

func pickPeriod(p period, base *url.URL, tallest int, want string) (video, audio Rep, ok bool) {
	var haveVideo, haveAudio bool
	audioRank := -1
	for _, a := range p.Sets {
		kind := a.Content
		if kind == "" {
			kind = strings.SplitN(a.Mime, "/", 2)[0]
		}
		set := resolve(base, a.Base)
		for _, r := range a.Reps {
			rep, ok := describe(set, a, r)
			if !ok {
				continue
			}
			switch kind {
			case "video":
				if !strings.HasPrefix(rep.Codecs, "avc1") || rep.Height > tallest {
					continue
				}
				if !haveVideo || rep.Height > video.Height || rep.Height == video.Height && rep.Bandwidth > video.Bandwidth {
					video, haveVideo = rep, true
				}
			case "audio":
				if !strings.HasPrefix(rep.Codecs, "mp4a") {
					continue
				}
				rank := 0
				if strings.HasPrefix(strings.ToLower(a.Lang), want) {
					rank += 2
				}
				if main(a) {
					rank++
				}
				if rank > audioRank || rank == audioRank && rep.Bandwidth > audio.Bandwidth {
					audio, haveAudio, audioRank = rep, true, rank
				}
			}
		}
	}
	return video, audio, haveVideo && haveAudio
}

func main(a adaptation) bool {
	for _, r := range a.Roles {
		if r.Value != "main" {
			return false
		}
	}
	return true
}

func resolve(u *url.URL, parts ...string) *url.URL {
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if r, err := url.Parse(p); err == nil {
			u = u.ResolveReference(r)
		}
	}
	return u
}

func describe(set *url.URL, a adaptation, r representation) (Rep, bool) {
	media := resolve(set, r.Base)
	rep := Rep{
		URL:   media.String(),
		Width: r.Width, Height: r.Height, Bandwidth: r.Bandwidth,
		Codecs: first(r.Codecs, a.Codecs), Lang: a.Lang,
	}
	for _, c := range a.Protection {
		if c.KID != "" {
			rep.KID = c.KID
		}
	}
	list := r.List
	if list == nil {
		list = a.List
	}
	if list != nil && list.Init != nil && len(list.URLs) > 0 {
		init, ok := span(list.Init.Range)
		if !ok {
			return Rep{}, false
		}
		rep.Init, rep.Index = init, init
		rep.InitURL, rep.IndexURL = rep.URL, rep.URL
		rep.Offset = scaled(list.Offset, list.Timescale)
		rep.Frags = fragments(list)
		return rep, rep.Frags != nil
	}
	seg := r.Segment
	if seg == nil {
		seg = a.Segment
	}
	if seg == nil || seg.Init == nil {
		return Rep{}, false
	}
	init, ok := span(seg.Init.Range)
	if !ok {
		return Rep{}, false
	}
	rep.Init, rep.InitURL, rep.IndexURL = init, rep.URL, rep.URL
	rep.Offset = scaled(seg.Offset, seg.Timescale)
	if seg.Init.Source != "" {
		rep.InitURL = resolve(media, seg.Init.Source).String()
	}
	switch {
	case seg.Index != "":
		index, ok := span(seg.Index)
		if !ok {
			return Rep{}, false
		}
		rep.Index = index
	case seg.Represented != nil:
		index, ok := span(seg.Represented.Range)
		if !ok {
			return Rep{}, false
		}
		rep.Index = index
		if seg.Represented.Source != "" {
			rep.IndexURL = resolve(media, seg.Represented.Source).String()
		}
	default:
		rep.Index = [2]int64{-1, -1}
	}
	return rep, true
}

func fragments(l *segmentList) []cenc.Fragment {
	scale := l.Timescale
	if scale <= 0 {
		scale = 1
	}
	var durations []int64
	for _, s := range l.Timeline {
		for range s.R + 1 {
			durations = append(durations, s.D)
		}
	}
	out := make([]cenc.Fragment, 0, len(l.URLs))
	var t int64
	for i, u := range l.URLs {
		r, ok := span(u.Range)
		if !ok {
			return nil
		}
		d := l.Duration
		if i < len(durations) {
			d = durations[i]
		}
		out = append(out, cenc.Fragment{
			Offset:   r[0],
			Size:     r[1] - r[0] + 1,
			At:       time.Duration(t) * time.Second / time.Duration(scale),
			Duration: time.Duration(d) * time.Second / time.Duration(scale),
		})
		t += d
	}
	return out
}

func span(s string) ([2]int64, bool) {
	from, to, ok := strings.Cut(s, "-")
	if !ok {
		return [2]int64{}, false
	}
	a, err1 := strconv.ParseInt(from, 10, 64)
	b, err2 := strconv.ParseInt(to, 10, 64)
	return [2]int64{a, b}, err1 == nil && err2 == nil && b >= a
}

func Length(data []byte) time.Duration {
	var m mpd
	if xml.Unmarshal(data, &m) != nil {
		return 0
	}
	return ParseDuration(m.Duration)
}

var protectionTag = regexp.MustCompile(`(?s)<ContentProtection.*?(/>|</ContentProtection>)`)

func excerpt(data []byte) string {
	s := protectionTag.ReplaceAllString(string(data), "")
	i := strings.Index(s, "<AdaptationSet")
	if i < 0 {
		i = 0
	}
	s = s[i:]
	if j := strings.Index(s, "</Representation>"); j > 0 {
		s = s[:j+len("</Representation>")]
	}
	if len(s) > 1500 {
		s = s[:1500]
	}
	return s
}

func scaled(v, scale int64) time.Duration {
	if scale <= 0 {
		scale = 1
	}
	return time.Duration(v) * time.Second / time.Duration(scale)
}

func Describe(parts []Part) string {
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString(" | ")
		}
		fmt.Fprintf(&b, "%d id=%q at=%s for=%s video=%dx%d kid=%s offset=%s audio=%s kid=%s base=%s",
			i, p.ID, p.Start.Round(time.Millisecond), p.Duration.Round(time.Millisecond),
			p.Video.Width, p.Video.Height, p.Video.KID, p.Video.Offset, p.Audio.Lang, p.Audio.KID, path.Dir(urlPath(p.Video.URL)))
	}
	return b.String()
}

func urlPath(s string) string {
	if u, err := url.Parse(s); err == nil {
		return u.Path
	}
	return s
}

func (p Part) Encrypted() bool { return p.Video.KID != "" || p.Audio.KID != "" }

func Bumper(parts []Part) (time.Duration, bool) {
	if len(parts) < 2 || parts[0].Encrypted() {
		return 0, false
	}
	for _, p := range parts[1:] {
		if p.Encrypted() {
			return p.Start, true
		}
	}
	return 0, false
}
