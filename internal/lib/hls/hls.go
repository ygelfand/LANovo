package hls

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/asticode/go-astits"
	"github.com/bluenviron/gohlslib/v2/pkg/playlist"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	tscodecs "github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts/codecs"
	"github.com/tphakala/go-aac/pcm"
)

type Options struct {
	HTTP     *http.Client
	Ask      func(*http.Request)
	Tallest  int
	Fastest  int
	Lead     time.Duration
	From     time.Time
	Resample func(from int) func(stereo []int16) []int16
}

type Frame struct {
	Data []byte
	At   time.Duration
}

type Stream struct {
	Width, Height int

	o      Options
	cancel context.CancelFunc
	done   chan struct{}
	sized  chan struct{}
	once   sync.Once

	mu       sync.Mutex
	room     *sync.Cond
	pcm      []int16
	err      error
	origin   time.Duration
	anchored chan struct{}
	anchor   sync.Once
	began    time.Time

	oldest, edge, seen time.Time

	wall     time.Time
	dec      *pcm.FrameDecoder
	channels int
	resample func([]int16) []int16
	decoded  []byte

	frames chan Frame
	sps    []byte
	pps    []byte
	keyed  bool
}

const (
	heldAudio     = 60 * time.Second
	heldFrames    = 300
	sizeWait      = 20 * time.Second
	outRate       = 48000
	startDistance = 3
	playlistMost  = 64 << 20
	segmentMost   = 32 << 20
	tsClock       = 90000
)

func Open(ctx context.Context, master string, o Options) (*Stream, error) {
	mv, err := fetchMaster(ctx, master, o)
	if err != nil {
		return nil, err
	}
	n, err := narrowed(mv, o)
	if err != nil {
		return nil, err
	}
	v := n.Variants[0]
	video, err := resolve(master, v.URI)
	if err != nil {
		return nil, err
	}
	audio, name := "", "muxed"
	if len(n.Renditions) > 0 && n.Renditions[0].URI != nil {
		if audio, err = resolve(master, *n.Renditions[0].URI); err != nil {
			return nil, err
		}
		name = n.Renditions[0].Name
	}
	slog.Info("hls variant", "resolution", v.Resolution, "bandwidth", v.Bandwidth, "codecs", strings.Join(v.Codecs, ","), "audio", name)

	run, cancel := context.WithCancel(context.Background())
	s := &Stream{
		o:        o,
		cancel:   cancel,
		done:     make(chan struct{}),
		sized:    make(chan struct{}),
		anchored: make(chan struct{}),
		frames:   make(chan Frame, heldFrames),
	}
	s.room = sync.NewCond(&s.mu)
	go func() { s.fail(s.follow(run, video, true, audio == "")) }()
	if audio != "" {
		go func() { s.fail(s.follow(run, audio, false, true)) }()
	}

	wait := time.NewTimer(sizeWait)
	defer wait.Stop()
	select {
	case <-s.sized:
		return s, nil
	case <-s.done:
		s.Close()
		return nil, s.failure()
	case <-ctx.Done():
		s.Close()
		return nil, ctx.Err()
	case <-wait.C:
		s.Close()
		return nil, errors.New("hls: no picture arrived")
	}
}

func (s *Stream) fail(err error) {
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
	s.stop()
}

func (s *Stream) stop() {
	s.once.Do(func() {
		s.cancel()
		close(s.done)
		s.mu.Lock()
		s.room.Broadcast()
		s.mu.Unlock()
	})
}

func (s *Stream) failure() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		return io.EOF
	}
	return s.err
}

func (s *Stream) Close() { s.fail(io.EOF) }

func (s *Stream) Read(out []int16) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := copy(out, s.pcm)
	s.pcm = s.pcm[n:]
	if n > 0 {
		s.room.Broadcast()
		return n, nil
	}
	select {
	case <-s.done:
		return 0, s.err
	default:
		return 0, nil
	}
}

func (s *Stream) Next() (Frame, error) {
	select {
	case <-s.anchored:
	case <-s.done:
		return Frame{}, s.failure()
	}
	select {
	case f := <-s.frames:
		f.At = max(f.At-s.origin, 0)
		return f, nil
	case <-s.done:
		return Frame{}, s.failure()
	}
}

func (s *Stream) follow(ctx context.Context, u string, video, audio bool) error {
	p := &pace{name: "audio", since: time.Now()}
	if video {
		p.name = "video"
	}
	next := -1
	progress := time.Now()
	stall := stallLeast
	for {
		began := time.Now()
		m, final, err := s.media(ctx, u)
		if err != nil {
			if ctx.Err() != nil || time.Since(progress) > stall {
				return err
			}
			slog.Warn("hls playlist", "track", p.name, "err", err)
			if err := pause(ctx, missWait*4); err != nil {
				return err
			}
			continue
		}
		p.playlist(time.Since(began))
		u = final
		stall = max(stallLeast, time.Duration(m.TargetDuration)*stallTargets*time.Second)
		first := m.MediaSequence
		last := first + len(m.Segments) - 1
		walls := clock(m)
		s.observe(m, walls)
		if next < first {
			if next >= 0 {
				slog.Warn("hls fell behind the live window", "track", p.name, "wanted", next, "oldest", first)
			}
			next = first + start(m, walls, s.o.Lead, s.o.From)
		}
		template, at := "", -1
		var wall time.Time
		for ; next <= last; next++ {
			seg := m.Segments[next-first]
			su, err := resolve(u, seg.URI)
			if err != nil {
				return err
			}
			got, err := s.take(ctx, su, walls[next-first], video, audio, p)
			if err != nil {
				return fmt.Errorf("hls: segment %d: %w", next, err)
			}
			if !got {
				break
			}
			progress, template, at = time.Now(), su, next
			if w := walls[next-first]; !w.IsZero() {
				wall = w.Add(seg.Duration)
			}
			p.report(s)
		}
		if m.Endlist && next > last {
			return io.EOF
		}
		if quiet := time.Since(progress); quiet > stall {
			slog.Info("hls stream stopped", "track", p.name, "quiet", quiet.Round(time.Second), "next", next)
			return io.EOF
		}
		if at < 0 {
			if at = next - 1; at >= first && at <= last {
				template, _ = resolve(u, m.Segments[at-first].URI)
				if w := walls[at-first]; !w.IsZero() {
					wall = w.Add(m.Segments[at-first].Duration)
				}
			}
		}
		step := time.Duration(m.TargetDuration) * time.Second
		if len(m.Segments) > 0 {
			step = m.Segments[len(m.Segments)-1].Duration
		}
		if err := s.ahead(ctx, template, at, &next, &progress, &wall, step, stall, video, audio, p, m.TargetDuration); err != nil {
			return err
		}
		if err := pause(ctx, max(time.Duration(m.TargetDuration)*time.Second/8, missWait)); err != nil {
			return err
		}
	}
}

func pause(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

const (
	liveLead  = 10 * time.Second
	missWait  = 250 * time.Millisecond
	paceEvery = 30 * time.Second
)

var (
	stallLeast   = 30 * time.Second
	stallTargets = time.Duration(6)
)

func start(m *playlist.Media, walls []time.Time, want time.Duration, from time.Time) int {
	if want <= 0 {
		want = liveLead
	}
	var held time.Duration
	i := len(m.Segments) - 1
	for ; i > 0 && (held < want || len(m.Segments)-i < startDistance); i-- {
		held += m.Segments[i].Duration
	}
	i = max(i, 0)
	if from.IsZero() {
		return i
	}
	for j := range i {
		w := walls[j]
		if !w.IsZero() && from.Before(w.Add(m.Segments[j].Duration)) {
			return j
		}
	}
	return i
}

func clock(m *playlist.Media) []time.Time {
	walls := make([]time.Time, len(m.Segments))
	var at time.Time
	for i, seg := range m.Segments {
		if seg.DateTime != nil {
			at = *seg.DateTime
		}
		walls[i] = at
		if !at.IsZero() {
			at = at.Add(seg.Duration)
		}
	}
	return walls
}

func (s *Stream) observe(m *playlist.Media, walls []time.Time) {
	if len(walls) == 0 || walls[0].IsZero() || m.Endlist {
		return
	}
	last := len(walls) - 1
	s.mu.Lock()
	s.oldest, s.edge, s.seen = walls[0], walls[last].Add(m.Segments[last].Duration), time.Now()
	s.mu.Unlock()
}

func (s *Stream) Window() (oldest, edge time.Time, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen.IsZero() {
		return time.Time{}, time.Time{}, false
	}
	since := time.Since(s.seen)
	return s.oldest.Add(since), s.edge.Add(since), true
}

func (s *Stream) Began() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.began
}

func (s *Stream) ahead(ctx context.Context, template string, at int, next *int, progress, wall *time.Time, step, stall time.Duration, video, audio bool, p *pace, target int) error {
	mark := fmt.Sprintf("/sq/%d/", at)
	if at < 0 || !strings.Contains(template, mark) {
		return nil
	}
	misses := 0
	limit := max(int(time.Duration(max(target, 1))*3*time.Second/missWait), 8)
	for misses < limit && time.Since(*progress) <= stall {
		u := strings.Replace(template, mark, fmt.Sprintf("/sq/%d/", *next), 1)
		got, err := s.take(ctx, u, *wall, video, audio, p)
		if err != nil {
			return fmt.Errorf("hls: segment %d: %w", *next, err)
		}
		if !got {
			misses++
			p.misses++
			if err := pause(ctx, missWait); err != nil {
				return err
			}
			continue
		}
		misses = 0
		*next++
		*progress = time.Now()
		if !wall.IsZero() {
			*wall = wall.Add(step)
		}
		p.report(s)
	}
	return nil
}

func (s *Stream) take(ctx context.Context, u string, wall time.Time, video, audio bool, p *pace) (bool, error) {
	began := time.Now()
	data, err := s.get(ctx, u, segmentMost)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		slog.Debug("hls segment not there", "err", err)
		return false, nil
	}
	p.segment(time.Since(began))
	return true, s.segment(data, video, audio, wall)
}

type statusError struct {
	code   int
	status string
}

func (e *statusError) Error() string { return e.status }

type pace struct {
	name                  string
	since                 time.Time
	playlists, segments   int
	playlistSum, segSum   time.Duration
	playlistMost, segMost time.Duration
	misses                int
}

func (p *pace) playlist(d time.Duration) {
	p.playlists++
	p.playlistSum += d
	p.playlistMost = max(p.playlistMost, d)
}

func (p *pace) segment(d time.Duration) {
	p.segments++
	p.segSum += d
	p.segMost = max(p.segMost, d)
}

func (p *pace) report(s *Stream) {
	if time.Since(p.since) < paceEvery {
		return
	}
	mean := func(sum time.Duration, n int) time.Duration {
		if n == 0 {
			return 0
		}
		return (sum / time.Duration(n)).Round(time.Millisecond)
	}
	slog.Info("hls pace", "track", p.name,
		"playlists", p.playlists, "playlist_mean", mean(p.playlistSum, p.playlists), "playlist_max", p.playlistMost.Round(time.Millisecond),
		"segments", p.segments, "segment_mean", mean(p.segSum, p.segments), "segment_max", p.segMost.Round(time.Millisecond),
		"misses", p.misses, "buffered", s.buffered().Round(100*time.Millisecond))
	*p = pace{name: p.name, since: time.Now()}
}

func (s *Stream) buffered() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Duration(len(s.pcm)/2) * time.Second / outRate
}

func (s *Stream) media(ctx context.Context, u string) (*playlist.Media, string, error) {
	body, final, err := s.fetch(ctx, u, playlistMost)
	if err != nil {
		return nil, "", fmt.Errorf("hls: playlist: %w", err)
	}
	pl, err := playlist.Unmarshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("hls: playlist: %w", err)
	}
	m, ok := pl.(*playlist.Media)
	if !ok {
		return nil, "", errors.New("hls: playlist: not a media playlist")
	}
	return m, final, nil
}

func (s *Stream) get(ctx context.Context, u string, most int64) ([]byte, error) {
	body, _, err := s.fetch(ctx, u, most)
	return body, err
}

func (s *Stream) fetch(ctx context.Context, u string, most int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	if s.o.Ask != nil {
		s.o.Ask(req)
	}
	resp, err := client(s.o).Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", &statusError{code: resp.StatusCode, status: resp.Status}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, most))
	return body, resp.Request.URL.String(), err
}

func resolve(base, ref string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	r, err := b.Parse(ref)
	if err != nil {
		return "", err
	}
	return r.String(), nil
}

func (s *Stream) segment(data []byte, video, audio bool, wall time.Time) error {
	if audio {
		s.wall = wall
	}
	switch {
	case len(data) > 0 && data[0] == 0x47:
		return s.transport(data, video, audio)
	case audio:
		return s.packed(data)
	default:
		return fmt.Errorf("unknown segment format, starts % x", data[:min(len(data), 4)])
	}
}

func (s *Stream) transport(data []byte, video, audio bool) error {
	r := &mpegts.Reader{R: bytes.NewReader(data)}
	if err := r.Initialize(); err != nil {
		return err
	}
	r.OnDecodeError(func(err error) { slog.Debug("hls ts", "err", err) })
	for _, t := range r.Tracks() {
		switch c := t.Codec.(type) {
		case *tscodecs.H264:
			if video {
				r.OnDataH264(t, func(pts, _ int64, au [][]byte) error {
					s.picture(ticks(pts), au)
					return nil
				})
			}
		case *tscodecs.MPEG4Audio:
			if audio {
				cfg := c.Config
				r.OnDataMPEG4Audio(t, func(pts int64, aus [][]byte) error { return s.aac(cfg, ticks(pts), aus) })
			}
		}
	}
	for {
		if err := r.Read(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func (s *Stream) packed(data []byte) error {
	pts, rest, ok := id3Timestamp(data)
	if !ok {
		return errors.New("packed audio without a timestamp")
	}
	var pkts mpeg4audio.ADTSPackets
	if err := pkts.Unmarshal(rest); err != nil {
		return err
	}
	if len(pkts) == 0 {
		return nil
	}
	aus := make([][]byte, len(pkts))
	for i, p := range pkts {
		aus[i] = p.AU
	}
	cfg := mpeg4audio.AudioSpecificConfig{Type: pkts[0].Type, SampleRate: pkts[0].SampleRate, ChannelConfig: pkts[0].ChannelConfig}
	return s.aac(cfg, ticks(pts), aus)
}

const tsTimestampOwner = "com.apple.streaming.transportStreamTimestamp"

func id3Timestamp(b []byte) (int64, []byte, bool) {
	var pts int64
	found := false
	for len(b) >= 10 && string(b[:3]) == "ID3" {
		size := syncsafe(b[6:10])
		end := 10 + size
		if b[5]&0x10 != 0 {
			end += 10
		}
		if end > len(b) {
			return 0, nil, false
		}
		frames := b[10 : 10+size]
		for len(frames) >= 10 {
			fs := int(binary.BigEndian.Uint32(frames[4:8]))
			if b[3] >= 4 {
				fs = syncsafe(frames[4:8])
			}
			if fs <= 0 || 10+fs > len(frames) {
				break
			}
			if string(frames[:4]) == "PRIV" {
				owner, val, ok := bytes.Cut(frames[10:10+fs], []byte{0})
				if ok && string(owner) == tsTimestampOwner && len(val) >= 8 {
					pts, found = int64(binary.BigEndian.Uint64(val)&(1<<33-1)), true
				}
			}
			frames = frames[10+fs:]
		}
		b = b[end:]
	}
	return pts, b, found
}

func syncsafe(b []byte) int {
	return int(b[0]&0x7f)<<21 | int(b[1]&0x7f)<<14 | int(b[2]&0x7f)<<7 | int(b[3]&0x7f)
}

func (s *Stream) aac(cfg mpeg4audio.AudioSpecificConfig, at time.Duration, aus [][]byte) error {
	if s.dec == nil {
		asc, err := cfg.Marshal()
		if err != nil {
			return err
		}
		dec, err := pcm.NewRawDecoder(asc)
		if err != nil {
			return fmt.Errorf("audio: %w", err)
		}
		s.dec, s.channels = dec, dec.Channels()
		s.resample = func(p []int16) []int16 { return p }
		if s.o.Resample != nil && dec.SampleRate() != outRate {
			s.resample = s.o.Resample(dec.SampleRate())
		}
		slog.Info("hls audio", "rate", dec.SampleRate(), "channels", s.channels)
		s.anchor.Do(func() {
			s.origin = at
			s.mu.Lock()
			s.began = s.wall
			s.mu.Unlock()
			close(s.anchored)
		})
	}
	for _, au := range aus {
		out, _, err := s.dec.DecodeFrame(s.decoded[:0], au)
		if err != nil {
			continue
		}
		s.decoded = out
		s.sound(s.resample(stereo(out, s.channels)))
	}
	return nil
}

func ticks(pts int64) time.Duration {
	return time.Duration(pts) * time.Second / tsClock
}

func stereo(b []byte, channels int) []int16 {
	n := len(b) / 2
	if channels == 1 {
		out := make([]int16, 2*n)
		for i := range n {
			v := int16(binary.LittleEndian.Uint16(b[2*i:]))
			out[2*i], out[2*i+1] = v, v
		}
		return out
	}
	out := make([]int16, n)
	for i := range n {
		out[i] = int16(binary.LittleEndian.Uint16(b[2*i:]))
	}
	return out
}

func (s *Stream) sound(p []int16) {
	limit := int(heldAudio/time.Second) * outRate * 2
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.pcm) > limit {
		select {
		case <-s.done:
			return
		default:
		}
		s.room.Wait()
	}
	s.pcm = append(s.pcm, p...)
}

func (s *Stream) picture(at time.Duration, au [][]byte) {
	var sps, pps bool
	for _, n := range au {
		if len(n) == 0 {
			continue
		}
		switch h264.NALUType(n[0] & 0x1f) {
		case h264.NALUTypeSPS:
			s.sps, sps = n, true
		case h264.NALUTypePPS:
			s.pps, pps = n, true
		}
	}
	if sps {
		s.size()
	}
	key := h264.IsRandomAccess(au)
	if !s.keyed && !key {
		return
	}
	if key && s.sps != nil && s.pps != nil {
		if !pps {
			au = append([][]byte{s.pps}, au...)
		}
		if !sps {
			au = append([][]byte{s.sps}, au...)
		}
	}
	data, err := h264.AnnexB(au).Marshal()
	if err != nil {
		return
	}
	select {
	case s.frames <- Frame{Data: data, At: at}:
		s.keyed = true
	case <-s.done:
	default:
		s.keyed = false
	}
}

func (s *Stream) size() {
	if s.Width > 0 || s.sps == nil {
		return
	}
	var p h264.SPS
	if err := p.Unmarshal(s.sps); err != nil {
		return
	}
	s.Width, s.Height = p.Width(), p.Height()
	close(s.sized)
}

func fetchMaster(ctx context.Context, master string, o Options) (*playlist.Multivariant, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, master, nil)
	if err != nil {
		return nil, err
	}
	if o.Ask != nil {
		o.Ask(req)
	}
	resp, err := client(o).Do(req)
	if err != nil {
		return nil, fmt.Errorf("hls: fetching the playlist: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hls: fetching the playlist: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, playlistMost))
	if err != nil {
		return nil, err
	}
	pl, err := playlist.Unmarshal(body)
	if err != nil {
		return nil, fmt.Errorf("hls: %w", err)
	}
	mv, ok := pl.(*playlist.Multivariant)
	if !ok {
		return nil, errors.New("hls: not a multivariant playlist")
	}
	return mv, nil
}

func client(o Options) *http.Client {
	if o.HTTP == nil {
		return http.DefaultClient
	}
	return o.HTTP
}

func narrowed(mv *playlist.Multivariant, o Options) (*playlist.Multivariant, error) {
	v := pick(mv.Variants, o.Tallest, o.Fastest)
	if v == nil {
		return nil, errors.New("hls: no H.264 and AAC-LC variant within the size limits")
	}
	out := &playlist.Multivariant{Version: mv.Version, IndependentSegments: mv.IndependentSegments, Variants: []*playlist.MultivariantVariant{v}}
	if r := rendition(mv.Renditions, v.Audio); r != nil {
		out.Renditions = []*playlist.MultivariantRendition{r}
	}
	return out, nil
}

func rendition(rs []*playlist.MultivariantRendition, group string) *playlist.MultivariantRendition {
	if group == "" {
		return nil
	}
	var first *playlist.MultivariantRendition
	for _, r := range rs {
		if r.Type != playlist.MultivariantRenditionTypeAudio || r.GroupID != group {
			continue
		}
		if r.Default {
			return r
		}
		if first == nil {
			first = r
		}
	}
	return first
}

func Describe(ctx context.Context, master string, o Options) (string, error) {
	mv, err := fetchMaster(ctx, master, o)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	chosen, _ := narrowed(mv, o)
	for _, v := range mv.Variants {
		mark := " "
		if chosen != nil && chosen.Variants[0] == v {
			mark = "*"
		}
		fps := ""
		if v.FrameRate != nil {
			fps = strconv.FormatFloat(*v.FrameRate, 'f', -1, 64)
		}
		fmt.Fprintf(&b, "%s variant %-10s %4sfps %9d bps %-28s audio=%s\n", mark, v.Resolution, fps, v.Bandwidth, strings.Join(v.Codecs, ","), v.Audio)
	}
	for _, r := range mv.Renditions {
		mark := " "
		if chosen != nil && len(chosen.Renditions) > 0 && chosen.Renditions[0] == r {
			mark = "*"
		}
		fmt.Fprintf(&b, "%s rendition %s group=%s name=%q lang=%s default=%v\n", mark, r.Type, r.GroupID, r.Name, r.Language, r.Default)
	}
	if chosen == nil {
		return b.String(), nil
	}
	uris := []string{chosen.Variants[0].URI}
	if len(chosen.Renditions) > 0 && chosen.Renditions[0].URI != nil {
		uris = append(uris, *chosen.Renditions[0].URI)
	}
	for _, u := range uris {
		peek(ctx, &b, master, u, o)
	}
	return b.String(), nil
}

func peek(ctx context.Context, b *strings.Builder, master, uri string, o Options) {
	base, err := url.Parse(master)
	if err != nil {
		return
	}
	u, err := base.Parse(uri)
	if err != nil {
		return
	}
	fmt.Fprintf(b, "playlist %s\n", tail(u.String()))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return
	}
	if o.Ask != nil {
		o.Ask(req)
	}
	resp, err := client(o).Do(req)
	if err != nil {
		fmt.Fprintf(b, "  %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, playlistMost))
	fmt.Fprintf(b, "  status %s, %d bytes, type %q, final %s, read err %v\n", resp.Status, len(body), resp.Header.Get("Content-Type"), tail(resp.Request.URL.String()), err)
	fmt.Fprintf(b, "  %d EXTINF lines\n", bytes.Count(body, []byte("#EXTINF:")))
	_, perr := playlist.Unmarshal(body)
	fmt.Fprintf(b, "  parse: %v\n", perr)
	lines := strings.SplitN(string(body), "\n", 13)
	for _, l := range lines[:min(len(lines), 12)] {
		if len(l) > 160 {
			l = l[:160] + "…"
		}
		fmt.Fprintf(b, "  | %s\n", l)
	}
	all := strings.Split(strings.TrimSpace(string(body)), "\n")
	if last := all[len(all)-1]; !strings.HasPrefix(last, "#") {
		if seg, err := u.Parse(last); err == nil {
			segment(ctx, b, seg.String(), o)
		}
	}
}

func segment(ctx context.Context, b *strings.Builder, u string, o Options) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return
	}
	if o.Ask != nil {
		o.Ask(req)
	}
	resp, err := client(o).Do(req)
	if err != nil {
		fmt.Fprintf(b, "  segment: %v\n", err)
		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	fmt.Fprintf(b, "  segment %s, %d bytes, type %q, read err %v\n", resp.Status, len(data), resp.Header.Get("Content-Type"), err)
	fmt.Fprintf(b, "  starts % x %q\n", data[:min(len(data), 24)], data[:min(len(data), 24)])
	if pts, rest, ok := id3Timestamp(data); ok {
		var pkts mpeg4audio.ADTSPackets
		perr := pkts.Unmarshal(rest)
		fmt.Fprintf(b, "  packed audio at %v, %d ADTS frames, err %v\n", ticks(pts), len(pkts), perr)
		return
	}
	dmx := astits.NewDemuxer(ctx, bytes.NewReader(data))
	pes := map[uint16]int{}
	for {
		d, err := dmx.NextData()
		if err != nil {
			fmt.Fprintf(b, "  demux ended: %v\n", err)
			break
		}
		if d.PMT != nil {
			for _, es := range d.PMT.ElementaryStreams {
				fmt.Fprintf(b, "  pmt pid %d type 0x%02x\n", es.ElementaryPID, uint8(es.StreamType))
			}
		}
		if d.PES != nil {
			pes[d.PID]++
		}
	}
	for pid, n := range pes {
		fmt.Fprintf(b, "  pes pid %d: %d\n", pid, n)
	}
}

func tail(u string) string {
	u, _, _ = strings.Cut(u, "?")
	parts := strings.Split(u, "/")
	return strings.Join(parts[max(len(parts)-4, 0):], "/")
}

func pick(vs []*playlist.MultivariantVariant, tallest, fastest int) *playlist.MultivariantVariant {
	var best *playlist.MultivariantVariant
	for _, v := range vs {
		if !playable(v.Codecs) {
			continue
		}
		if tallest > 0 && height(v.Resolution) > tallest {
			continue
		}
		if fastest > 0 && v.FrameRate != nil && *v.FrameRate > float64(fastest)+0.5 {
			continue
		}
		if best == nil || v.Bandwidth > best.Bandwidth {
			best = v
		}
	}
	return best
}

func playable(cs []string) bool {
	var avc, aac bool
	for _, c := range cs {
		switch {
		case strings.HasPrefix(c, "avc1."):
			avc = true
		case c == "mp4a.40.2":
			aac = true
		}
	}
	return avc && aac
}

func height(res string) int {
	_, h, ok := strings.Cut(res, "x")
	if !ok {
		return 0
	}
	n, _ := strconv.Atoi(h)
	return n
}
