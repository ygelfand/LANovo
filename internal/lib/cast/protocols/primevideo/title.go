package primevideo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/cast/playback"
	"github.com/ygelfand/LANovo/internal/lib/cast/sealed"
	"github.com/ygelfand/LANovo/internal/lib/cenc"
)

const titleStartWait = 30 * time.Second

type title struct {
	proto    *Protocol
	id       string
	acct     Account
	envelope string
	handoff  string
	parts    []Part
	length   time.Duration
	from     time.Duration
	name     string
	series   string

	p      *sealed.Player
	cancel context.CancelFunc
}

func openTitle(ctx context.Context, proto *Protocol, t title) (*title, error) {
	var surf = proto.surface()
	nt := &t
	p, err := sealed.Open("prime video", surf, nil, nt.licensed, proto.env.Resample)
	if err != nil {
		return nil, err
	}
	nt.p = p
	run, cancel := context.WithCancel(context.Background())
	nt.cancel = cancel

	var wg sync.WaitGroup
	wg.Add(2)
	for _, video := range []bool{true, false} {
		go func() {
			defer wg.Done()
			err := nt.follow(run, video)
			slog.Info("prime video track ended", "video", video, "err", err)
			if err != nil && !errors.Is(err, io.EOF) && run.Err() == nil {
				nt.p.Fail(err)
			}
		}()
	}
	go func() {
		wg.Wait()
		nt.p.Fail(io.EOF)
	}()

	wait := time.NewTimer(titleStartWait)
	defer wait.Stop()
	select {
	case <-p.Ready():
		return nt, nil
	case <-p.Done():
		err := p.Failure()
		nt.close()
		return nil, err
	case <-ctx.Done():
		nt.close()
		return nil, ctx.Err()
	case <-wait.C:
		nt.close()
		return nil, errors.New("prime video: no picture arrived")
	}
}

func (t *title) licensed(ctx context.Context, challenge []byte) ([]byte, error) {
	return License(ctx, t.proto.env.HTTP, t.acct, t.id, t.envelope, t.handoff, challenge)
}

func (t *title) follow(ctx context.Context, video bool) error {
	opened := false
	for n, part := range t.parts {
		if part.Duration > 0 && part.Start+part.Duration <= t.from {
			continue
		}
		rep := part.Audio
		if video {
			rep = part.Video
		}
		if err := t.period(ctx, n, part, rep, video, !opened); err != nil {
			return err
		}
		opened = true
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return io.EOF
}

func (t *title) period(ctx context.Context, n int, part Part, rep Rep, video, first bool) error {
	hc := t.proto.env.HTTP
	init, err := Fetch(ctx, hc, rep.InitURL, rep.Init[0], rep.Init[1])
	if err != nil {
		return err
	}
	track, err := cenc.ParseInit(init)
	if err != nil {
		return fmt.Errorf("prime video: period %d init: %w", n, err)
	}
	frags := rep.Frags
	if frags == nil {
		if frags, err = t.index(ctx, rep); err != nil {
			return err
		}
	}
	var covered time.Duration
	if k := len(frags); k > 0 {
		covered = frags[k-1].At + frags[k-1].Duration - frags[0].At
	}
	slog.Info("prime video index", "period", n, "video", video, "fragments", len(frags), "covers", covered.Round(time.Second))
	if err := t.p.License(ctx, track.PSSH); err != nil {
		return err
	}
	if !video && first {
		if err := t.p.OpenAudio(track); err != nil {
			return err
		}
	}
	local := rep.Offset
	if t.from > part.Start {
		local += t.from - part.Start
	}
	i := 0
	for j, f := range frags {
		if f.At <= local {
			i = j
		}
	}
	for ; i < len(frags) && ctx.Err() == nil; i++ {
		f := frags[i]
		b, err := Fetch(ctx, hc, rep.URL, f.Offset, f.Offset+f.Size-1)
		if err != nil {
			return err
		}
		samples, pssh, err := cenc.ParseFragment(track, b)
		if err != nil {
			return fmt.Errorf("prime video: period %d fragment %d: %w", n, i, err)
		}
		if err := t.p.License(ctx, append(track.PSSH, pssh...)); err != nil {
			return err
		}
		for _, s := range samples {
			s.At = s.At - rep.Offset + part.Start
			if video {
				if err := t.p.Picture(ctx, track, s); err != nil {
					return err
				}
			} else if err := t.p.Sound(s); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

func (t *title) index(ctx context.Context, rep Rep) ([]cenc.Fragment, error) {
	if rep.Index[0] < 0 {
		return nil, fmt.Errorf("prime video: no index for %s", rep.URL)
	}
	b, err := Fetch(ctx, t.proto.env.HTTP, rep.IndexURL, rep.Index[0], rep.Index[1])
	if err != nil {
		return nil, err
	}
	var refs []cenc.Fragment
	if rep.IndexURL == rep.URL {
		refs, err = cenc.ParseIndex(b, rep.Index[0])
	} else {
		refs, err = cenc.ParseIndexAt(b, 0)
	}
	if err != nil {
		return nil, fmt.Errorf("prime video: index at %s %v: %w (%s)", rep.IndexURL, rep.Index, err, cenc.Boxes(b))
	}
	return t.expand(ctx, rep.URL, refs, 0)
}

func (t *title) expand(ctx context.Context, media string, refs []cenc.Fragment, depth int) ([]cenc.Fragment, error) {
	var out []cenc.Fragment
	for _, r := range refs {
		if !r.Index {
			out = append(out, r)
			continue
		}
		if depth >= 4 {
			return nil, errors.New("prime video: index nested too deep")
		}
		b, err := Fetch(ctx, t.proto.env.HTTP, media, r.Offset, r.Offset+r.Size-1)
		if err != nil {
			return nil, err
		}
		nested, err := cenc.ParseIndex(b, r.Offset)
		if err != nil {
			return nil, fmt.Errorf("prime video: index reference at %d+%d is not an index: %w (%s)", r.Offset, r.Size, err, cenc.Boxes(b))
		}
		more, err := t.expand(ctx, media, nested, depth+1)
		if err != nil {
			return nil, err
		}
		out = append(out, more...)
	}
	return out, nil
}

func (t *title) close() {
	t.cancel()
	t.p.Close()
}

func (t *title) Read(pcm []int16) (int, error) { return t.p.Read(pcm) }

func (t *title) Start() time.Duration {
	if at, ok := t.p.Origin(); ok {
		return at
	}
	return t.from
}

func (t *title) Showing() playback.Showing {
	t.proto.mu.Lock()
	defer t.proto.mu.Unlock()
	return playback.Showing{Title: first(t.name, "Prime Video"), Artist: t.series, Length: t.length}
}

func (t *title) Label() string { return "Prime Video" }

func (t *title) Play()     { go t.proto.play(t) }
func (t *title) Pause()    { go t.proto.pause(t) }
func (t *title) Stop()     { go t.proto.stop(t) }
func (t *title) Next()     {}
func (t *title) Previous() {}

func (t *title) Seek(to time.Duration) { go t.proto.seek(t, max(to, 0)) }
func (t *title) CanSeek() bool         { return true }

func (t *title) Pictured() bool { return true }
func (t *title) Pictures(context.Context) (io.ReadCloser, time.Duration, error) {
	return nil, 0, nil
}
func (t *title) Live(context.Context) (playback.Picture, error) {
	v := t.main().Video
	return t.p.Video(v.Width, v.Height), nil
}

func (t *title) main() Part {
	best := t.parts[0]
	for _, p := range t.parts {
		if p.Duration > best.Duration {
			best = p
		}
	}
	return best
}

func (t *title) Waiting(w bool) {
	go t.proto.buffering(t, w)
}

var isoDuration = regexp.MustCompile(`^PT(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?$`)

func ParseDuration(s string) time.Duration {
	m := isoDuration.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	var d time.Duration
	for i, unit := range []time.Duration{time.Hour, time.Minute, time.Second} {
		if m[i+1] == "" {
			continue
		}
		v, _ := strconv.ParseFloat(m[i+1], 64)
		d += time.Duration(v * float64(unit))
	}
	return d
}

func logTitle(t *title) {
	m := t.main()
	slog.Info("prime video playing", "title", t.id,
		"video", fmt.Sprintf("%dx%d@%dk %s", m.Video.Width, m.Video.Height, m.Video.Bandwidth/1000, m.Video.Codecs),
		"audio", fmt.Sprintf("%s %dk %s", m.Audio.Lang, m.Audio.Bandwidth/1000, m.Audio.Codecs),
		"length", t.length.Round(time.Second), "from", t.from.Round(time.Second), "periods", len(t.parts))
	slog.Info("prime video periods", "title", t.id, "parts", Describe(t.parts))
}
