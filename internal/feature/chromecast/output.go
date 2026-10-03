package chromecast

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/videoplayer"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/hardware/video"
	"github.com/ygelfand/LANovo/internal/lib/cast/playback"
	"github.com/ygelfand/LANovo/internal/lib/fetch"
	"github.com/ygelfand/LANovo/internal/lib/safe"
	"github.com/ygelfand/LANovo/internal/lib/surface"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Queued caps how far decoding may run ahead of the speaker. The speaker's queue is the buffer; this
// only keeps an hour-long stream from being decoded into memory whole.
const Queued = time.Minute

// ducked is the level under a voice turn, the same as the other background sources.
const ducked = 0.2

var _ playback.Output = (*output)(nil)
var _ speaker.Producer = (*output)(nil)

// output plays one source at a time through the speaker and on the player card.
type output struct {
	q    queue
	play func(context.Context, video.Stream) (video.Report, error)

	view      *videoplayer.Page
	viewStop  context.CancelFunc
	under     *video.Beneath
	viewGen   int
	viewTimer *time.Timer
	handing   bool
	handed    *card

	feed sync.Mutex

	mu      sync.Mutex
	ended   func()
	cur     *playing
	lastSrc playback.Source
	lastAt  time.Duration
	gain    float32
	down    bool
	sess    *playback.Session
	note    string
	sessArt *ui.Image
	standby *standby
}

type queue interface {
	Play([]int16)
	Take() []int16
	Queued() int
	Drain()
	Adjust(func([]int16))
}

type playing struct {
	src  playback.Source
	card *card
	stop context.CancelFunc

	kept    []int16
	paused  bool
	waiting bool
	fed     int64

	art     *ui.Image
	artURL  string
	mark    *ui.Image
	markURL string

	seen     chan struct{}
	seenOnce sync.Once
}

func (p *playing) shown() { p.seenOnce.Do(func() { close(p.seen) }) }

func newOutput() *output {
	o := &output{q: speaker.Get(), gain: 1}
	o.standby = &standby{o: o}
	return o
}

func (o *output) Play(src playback.Source) {
	o.release(nil)

	ctx, stop := context.WithCancel(context.Background())
	p := &playing{src: src, stop: stop}
	p.card = &card{o: o, p: p}
	pics, pictured := src.(playback.Pictures)
	pictured = pictured && pics.Pictured()
	if pictured {
		p.seen = make(chan struct{})
	}

	art := src.Showing().Art
	o.mu.Lock()
	o.cur, o.note = p, ""
	if h := o.handed; h != nil && art != "" && h.p.artURL == art {
		p.art, p.artURL = h.p.art, art
	}
	view := o.view
	o.mu.Unlock()
	if view != nil {
		view.SetNote("")
	}

	o.q.Drain()
	media.Get().Began(p.card)
	o.mu.Lock()
	o.handed = nil
	o.mu.Unlock()
	speaker.Sound().Backgrounds().Took(o)
	o.Changed(src)

	if pictured {
		if o.play == nil {
			view, _ := o.hold(p.card)
			if b, ok := src.(playback.Branded); ok && b.Logo() != "" {
				url := b.Logo()
				safe.Go("cast logo", func() {
					if img := o.logoFor(url); img != nil {
						view.SetLogo(img)
					}
				})
			}
		}
	}
	safe.Go("cast output", func() { o.pump(ctx, p) })
	if pictured {
		safe.Go("cast video", func() { o.picture(ctx, p, pics) })
	}
}

const viewGrace = 5 * time.Second

func (o *output) hold(c videoplayer.Controls) (*videoplayer.Page, *video.Beneath) {
	o.mu.Lock()
	o.viewGen++
	if o.viewTimer != nil {
		o.viewTimer.Stop()
		o.viewTimer = nil
	}
	slog.Info("cast video view held", "new", o.view == nil, "by", fmt.Sprintf("%T", c))
	if o.view == nil {
		ctx, stop := context.WithCancel(context.Background())
		o.view, o.viewStop, o.under = videoplayer.NewPage(c), stop, &video.Beneath{}
		view := o.view
		o.under.Stalled = func() { view.SetLoading(true) }
		o.under.Resumed = func() { view.SetLoading(false) }
		go view.Run(ctx)
	} else {
		o.view.Follow(c)
		o.view.SetLoading(true)
	}
	view, under := o.view, o.under
	o.mu.Unlock()
	return view, under
}

func (o *output) letGo() {
	o.mu.Lock()
	defer o.mu.Unlock()
	view := o.view
	if view == nil {
		return
	}
	if o.handing {
		go view.SetLoading(true)
	} else {
		go view.SetPicture(false)
	}
	gen := o.viewGen
	o.viewTimer = time.AfterFunc(viewGrace, func() {
		o.mu.Lock()
		if o.viewGen != gen || o.view == nil {
			o.mu.Unlock()
			return
		}
		stop, under := o.viewStop, o.under
		o.view, o.viewStop, o.under, o.viewTimer = nil, nil, nil, nil
		o.mu.Unlock()
		stop()
		under.Clear()
	})
}

const pictureWait = 30 * time.Second

func (o *output) picture(ctx context.Context, p *playing, pics playback.Pictures) {
	play := o.play
	var view *videoplayer.Page
	if play == nil {
		v, under := o.hold(p.card)
		view = v
		defer o.letGo()
		play = func(ctx context.Context, s video.Stream) (video.Report, error) { return video.On(ctx, under, s) }
	}
	err := o.pictureOnce(ctx, p, pics, play, view)
	if err == nil || ctx.Err() != nil {
		return
	}
	slog.Warn("cast video", "err", err)
	o.fail(err, true)
	o.release(p.src)
	safe.Go("cast stop", p.src.Stop)
}

var errNoPicture = errors.New("cast video: no picture")

func (o *output) pictureOnce(ctx context.Context, p *playing, pics playback.Pictures, play func(context.Context, video.Stream) (video.Report, error), view *videoplayer.Page) error {
	actx, cancel := context.WithCancel(ctx)
	defer cancel()
	var began, late atomic.Bool
	watch := time.AfterFunc(pictureWait, func() {
		if !began.Load() {
			late.Store(true)
			cancel()
		}
	})
	defer watch.Stop()

	s, closer, err := streamOf(actx, pics)
	if err != nil {
		return err
	}
	defer closer.Close()
	s.Clock = func() (time.Duration, bool) { return o.clock(p) }
	waiter, _ := p.src.(playback.Waiter)
	s.Started = func() {
		began.Store(true)
		p.shown()
		if waiter != nil {
			waiter.Waiting(false)
		}
		if view != nil {
			view.SetFrame(int(s.Width), int(s.Height))
			view.SetPicture(true)
		}
	}
	s.Waiting = func(w bool) {
		o.waiting(p, w)
		if waiter != nil {
			waiter.Waiting(w)
		}
		if view != nil {
			view.SetLoading(w)
		}
	}
	rep, err := play(actx, s)
	slog.Info("cast video over", "shown", rep.Shown, "dropped", rep.Dropped, "size", fmt.Sprintf("%dx%d", s.Width, s.Height), "err", err)
	if late.Load() {
		return errNoPicture
	}
	return err
}

func streamOf(ctx context.Context, pics playback.Pictures) (video.Stream, io.Closer, error) {
	if l, ok := pics.(playback.Live); ok {
		p, err := l.Live(ctx)
		if err != nil {
			return video.Stream{}, nil, err
		}
		if p.Sealed != nil {
			if !p.H264 {
				return video.Stream{}, nil, errNoPicture
			}
			s := video.Stream{Codec: video.H264, Width: uint32(p.Width), Height: uint32(p.Height), Session: p.Session, Source: sealedFrames(p.Sealed)}
			return s, io.NopCloser(nil), nil
		}
		if p.Next != nil {
			if !p.H264 {
				return video.Stream{}, nil, errNoPicture
			}
			s := video.Stream{Codec: video.H264, Width: uint32(p.Width), Height: uint32(p.Height), Source: liveFrames(p.Next)}
			return s, io.NopCloser(nil), nil
		}
	}
	stream, from, err := pics.Pictures(ctx)
	if err != nil {
		return video.Stream{}, nil, err
	}
	if stream == nil {
		return video.Stream{}, nil, errNoPicture
	}
	s, err := video.WebM(stream, from)
	if err != nil {
		stream.Close()
		return video.Stream{}, nil, err
	}
	return s, stream, nil
}

type liveFrames func() ([]byte, time.Duration, error)

func (f liveFrames) Next() (video.Frame, error) {
	data, at, err := f()
	return video.Frame{Data: data, At: at}, err
}

type sealedFrames func() ([]byte, time.Duration, *surface.Crypt, error)

func (f sealedFrames) Next() (video.Frame, error) {
	data, at, k, err := f()
	return video.Frame{Data: data, At: at, Crypt: k}, err
}

func (o *output) Stop(src playback.Source) {
	o.mu.Lock()
	o.handing = false
	o.mu.Unlock()
	o.release(src)
	o.dropHanded()
}

func (o *output) Handoff(src playback.Source) {
	o.mu.Lock()
	o.handing = true
	o.mu.Unlock()
	o.release(src)
}

// release gives up the card and the speaker if src is what is playing, or whatever is when src is nil.
func (o *output) release(src playback.Source) {
	o.mu.Lock()
	p := o.cur
	if p == nil || (src != nil && p.src != src) {
		o.mu.Unlock()
		return
	}
	o.cur = nil
	attended := src != nil && o.sess != nil && !o.handing
	handing := o.handing
	if handing {
		o.handed = p.card
	}
	o.mu.Unlock()

	p.stop()
	speaker.Sound().Backgrounds().Gave(o)
	switch {
	case attended:
		o.standBy()
	case !handing:
		media.Get().Release(p.card)
	}
	o.q.Drain()
}

func (o *output) dropHanded() {
	o.mu.Lock()
	c := o.handed
	o.handed = nil
	o.mu.Unlock()
	if c != nil {
		media.Get().Release(c)
	}
}

func (o *output) Attend(s *playback.Session) {
	o.mu.Lock()
	o.sess = s
	if s == nil {
		o.note, o.sessArt = "", nil
	}
	idle, held := o.cur == nil, o.view != nil
	o.mu.Unlock()
	if s == nil {
		media.Get().Ended(o.standby)
	}
	switch {
	case !idle:
	case s != nil:
		o.standBy()
		media.Get().Began(o.standby)
	default:
		media.Get().Release(o.standby)
		if held {
			o.letGo()
		}
	}
}

func (o *output) Failed(err error) { o.fail(err, false) }

func (o *output) end() {
	if o.ended != nil {
		safe.Go("cast end", o.ended)
	}
}

func (o *output) fail(err error, unseen bool) {
	o.dropHanded()
	o.mu.Lock()
	o.note = err.Error()
	view, idle := o.view, o.cur == nil
	o.mu.Unlock()
	if view != nil && (idle || unseen) {
		view.SetPicture(false)
		view.SetNote(err.Error())
	}
	media.Get().Changed()
}

func (o *output) standBy() {
	o.mu.Lock()
	s := o.sess
	o.mu.Unlock()
	if s == nil {
		return
	}
	media.Get().External(o.standby)
	if s.Logo == "" {
		return
	}
	safe.Go("cast logo", func() {
		img := o.logoFor(s.Logo)
		if img == nil {
			return
		}
		o.mu.Lock()
		if o.sess == s {
			o.sessArt = img
		}
		view, idle := o.view, o.cur == nil
		o.mu.Unlock()
		if view != nil && idle {
			view.SetLogo(img)
		}
		media.Get().Changed()
	})
}

func (o *output) Changed(src playback.Source) {
	o.mu.Lock()
	p := o.cur
	o.mu.Unlock()
	if p == nil || p.src != src {
		return
	}

	shown := src.Showing()

	o.mu.Lock()
	fetchArt := shown.Art != "" && shown.Art != p.artURL
	if fetchArt {
		p.artURL = shown.Art
	}
	fetchMark := shown.Mark != "" && shown.Mark != p.markURL
	if fetchMark {
		p.markURL = shown.Mark
	}
	o.mu.Unlock()

	if fetchArt {
		safe.Go("cast art", func() { o.fetchArt(p, shown.Art) })
	}
	if fetchMark {
		safe.Go("cast mark", func() {
			img := o.logoFor(shown.Mark)
			o.mu.Lock()
			if img != nil && o.cur == p && p.markURL == shown.Mark {
				p.mark = img
			}
			o.mu.Unlock()
			media.Get().Changed()
		})
	}
	media.Get().Changed()
}

func (o *output) Pause(src playback.Source) {
	o.feed.Lock()
	o.mu.Lock()
	p := o.cur
	if p == nil || p.src != src || p.paused {
		o.mu.Unlock()
		o.feed.Unlock()
		return
	}
	p.paused = true
	o.mu.Unlock()
	o.aside()
	o.feed.Unlock()
	media.Get().Changed()
}

func (o *output) Resume(src playback.Source) {
	o.feed.Lock()
	o.mu.Lock()
	p := o.cur
	if p == nil || p.src != src || !p.paused {
		o.mu.Unlock()
		o.feed.Unlock()
		return
	}
	p.paused = false
	hold := o.down || p.waiting
	o.mu.Unlock()
	if !hold {
		o.resume()
	}
	o.feed.Unlock()
	media.Get().Changed()
}

func (o *output) waiting(p *playing, w bool) {
	o.feed.Lock()
	defer o.feed.Unlock()
	o.mu.Lock()
	if o.cur != p || p.waiting == w {
		o.mu.Unlock()
		return
	}
	p.waiting = w
	hold := o.down || p.paused
	o.mu.Unlock()
	switch {
	case w:
		o.aside()
	case !hold:
		o.resume()
	}
}

func (o *output) clock(p *playing) (time.Duration, bool) {
	at, paused := o.Position(p.src)
	o.mu.Lock()
	waiting := p.waiting
	o.mu.Unlock()
	return at, !paused && !waiting
}

func (o *output) Position(src playback.Source) (time.Duration, bool) {
	o.mu.Lock()
	p := o.cur
	if p == nil || p.src != src {
		defer o.mu.Unlock()
		if src == o.lastSrc {
			return o.lastAt, false
		}
		return src.Start(), false
	}
	fed, kept, paused := p.fed, int64(len(p.kept)/speaker.Channels), p.paused
	o.mu.Unlock()
	heard := max(fed-kept-int64(o.q.Queued()), 0)
	at := src.Start() + time.Duration(heard)*time.Second/speaker.Rate
	o.mu.Lock()
	o.lastSrc, o.lastAt = src, at
	o.mu.Unlock()
	return at, paused
}

func (o *output) Behind(src playback.Source) time.Duration {
	o.mu.Lock()
	p := o.cur
	var kept int
	if p != nil {
		kept = len(p.kept) / speaker.Channels
	}
	o.mu.Unlock()
	if p == nil || p.src != src {
		return 0
	}
	frames := o.q.Queued() + kept
	return time.Duration(frames) * time.Second / speaker.Rate
}

// pump reads the source into the speaker until it ends or is replaced.
func (o *output) pump(ctx context.Context, p *playing) {
	buf := make([]int16, speaker.Rate/50*speaker.Channels)
	ahead := int(Queued * speaker.Rate / time.Second)

	if p.seen != nil {
		select {
		case <-p.seen:
		case <-ctx.Done():
			return
		}
	}
	for ctx.Err() == nil {
		if o.standing() || o.q.Queued() > ahead {
			wait(ctx, 100*time.Millisecond)
			continue
		}

		n, err := p.src.Read(buf)
		if n > 0 {
			out := make([]int16, n)
			copy(out, buf[:n])
			speaker.Scale(out, o.level())
			o.feed.Lock()
			o.mu.Lock()
			p.fed += int64(n / speaker.Channels)
			held := o.cur != p || o.down || p.paused || p.waiting
			if held {
				p.kept = append(p.kept, out...)
			}
			o.mu.Unlock()
			if !held {
				o.q.Play(out)
			}
			o.feed.Unlock()
		}
		if err != nil && ctx.Err() != nil {
			return
		}
		if errors.Is(err, io.EOF) {
			o.finish(ctx, p)
			return
		}
		if err != nil {
			slog.Warn("cast playback", "err", err)
			o.Failed(err)
			o.release(p.src)
			safe.Go("cast stop", p.src.Stop)
			return
		}
		if n == 0 {
			wait(ctx, 20*time.Millisecond)
		}
	}
}

// finish lets what is queued play out, then gives everything back.
func (o *output) finish(ctx context.Context, p *playing) {
	for ctx.Err() == nil && o.Behind(p.src) > 0 {
		wait(ctx, 100*time.Millisecond)
	}
	o.release(p.src)
}

func wait(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (o *output) standing() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.down || (o.cur != nil && (o.cur.paused || o.cur.waiting))
}

func (o *output) level() float32 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.gain
}

// aside takes what is queued out of the speaker and keeps it for later.
func (o *output) aside() {
	queued := o.q.Take()
	o.mu.Lock()
	if o.cur != nil {
		o.cur.kept = append(o.cur.kept, queued...)
	}
	o.mu.Unlock()
}

// resume puts back what was kept.
func (o *output) resume() {
	o.mu.Lock()
	var kept []int16
	if o.cur != nil {
		kept, o.cur.kept = o.cur.kept, nil
	}
	o.mu.Unlock()
	if len(kept) > 0 {
		o.q.Play(kept)
	}
}

// Stand implements speaker.Producer: a voice turn or another source standing this one down.
func (o *output) Stand(down bool) {
	o.feed.Lock()
	defer o.feed.Unlock()
	o.mu.Lock()
	first := down && !o.down
	back := !down && o.down && (o.cur == nil || (!o.cur.paused && !o.cur.waiting))
	o.down = down
	o.mu.Unlock()

	switch {
	case first:
		o.aside()
	case back:
		o.resume()
	}
}

// Duck implements speaker.Producer, setting the level for what is written next.
func (o *output) Duck(on bool) {
	gain := float32(1)
	if on {
		gain = ducked
	}
	o.mu.Lock()
	o.gain = gain
	o.mu.Unlock()
}

// Requeue implements speaker.Producer, scaling what is already queued.
func (o *output) Requeue() {
	gain := o.level()
	if gain == 1 {
		return
	}
	o.q.Adjust(func(samples []int16) { speaker.Scale(samples, gain) })
}

var (
	logoMu sync.Mutex
	logos  = map[string]*ui.Image{}
)

func (o *output) logoFor(url string) *ui.Image {
	logoMu.Lock()
	img, ok := logos[url]
	logoMu.Unlock()
	if ok {
		return img
	}
	img = fetchImage(url)
	if img != nil {
		logoMu.Lock()
		logos[url] = img
		logoMu.Unlock()
	}
	return img
}

func fetchImage(url string) *ui.Image {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	resp, err := fetch.Client(15 * time.Second).Do(req)
	if err != nil {
		slog.Debug("cast logo", "err", err)
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil
	}
	img, err := ui.Decode(body)
	if err != nil {
		slog.Debug("cast logo", "err", err)
		return nil
	}
	return img
}

func (o *output) fetchArt(p *playing, url string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	resp, err := fetch.Client(15 * time.Second).Do(req)
	if err != nil {
		slog.Debug("cast art", "err", err)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return
	}
	img, err := ui.Decode(body)
	if err != nil {
		slog.Debug("cast art", "err", err)
		return
	}

	o.mu.Lock()
	if o.cur == p && p.artURL == url {
		p.art = img
	}
	o.mu.Unlock()
	media.Get().Changed()
}

// card is a playing source as the player card sees it.
type card struct {
	o *output
	p *playing
}

var _ media.Source = (*card)(nil)

func (c *card) Now() media.Now {
	s := c.p.src.Showing()
	elapsed, paused := c.o.Position(c.p.src)

	c.o.mu.Lock()
	art, mark := c.p.art, c.p.mark
	c.o.mu.Unlock()

	can := media.CanPause | media.CanStop
	if s.HasNext {
		can |= media.CanNext
	}
	if s.HasPrevious {
		can |= media.CanPrevious
	}
	queue := make([]media.Track, 0, len(s.Upcoming))
	for _, u := range s.Upcoming {
		queue = append(queue, media.Track{Title: u.Title, Artist: u.Artist, Length: u.Length, Play: u.Play})
	}
	return media.Now{
		Queue:   queue,
		Playing: !paused,
		Paused:  paused,
		Title:   s.Title,
		Artist:  s.Artist,
		Album:   s.Album,
		Art:     art,
		Mark:    mark,
		Elapsed: elapsed,
		Length:  s.Length,

		LiveWithin: s.LiveWithin,
		Can:        can,
	}
}

func (c *card) Open() bool {
	c.o.mu.Lock()
	view, cur := c.o.view, c.o.cur
	c.o.mu.Unlock()
	if cur != c.p || c.p.seen == nil {
		return false
	}
	if view != nil {
		view.Show()
		return true
	}
	c.o.hold(c)
	return true
}

func (c *card) Kind() media.Kind { return media.FromCast }
func (c *card) Label() string    { return c.p.src.Label() }

func (c *card) Play()  { c.p.src.Play() }
func (c *card) Pause() { c.p.src.Pause() }

func (c *card) Stop() {
	c.p.src.Stop()
	c.o.end()
}
func (c *card) Next()     { c.p.src.Next() }
func (c *card) Previous() { c.p.src.Previous() }

func (c *card) Seek(to time.Duration) {
	if s, ok := c.p.src.(playback.Seeker); ok {
		s.Seek(to)
	}
}

func (c *card) Marks() []videoplayer.Mark {
	m, ok := c.p.src.(playback.Marked)
	if !ok {
		return nil
	}
	var out []videoplayer.Mark
	for _, k := range m.Marks() {
		out = append(out, videoplayer.Mark{From: k.From, To: k.To,
			Color: theme.Color{R: byte(k.RGB >> 16), G: byte(k.RGB >> 8), B: byte(k.RGB)}})
	}
	return out
}

func (c *card) CanSeek() bool {
	_, ok := c.p.src.(playback.Seeker)
	return ok && c.p.src.Showing().Length > 0
}

type standby struct{ o *output }

var _ media.Source = (*standby)(nil)

func (b *standby) Now() media.Now {
	b.o.mu.Lock()
	defer b.o.mu.Unlock()
	return media.Now{Hold: b.o.sess != nil, Title: b.o.note, Art: b.o.sessArt}
}

func (b *standby) Kind() media.Kind { return media.FromCast }

func (b *standby) Open() bool {
	b.o.mu.Lock()
	s, note, art, playing := b.o.sess, b.o.note, b.o.sessArt, b.o.cur != nil
	b.o.mu.Unlock()
	if s == nil || !s.Pictured || playing {
		return false
	}
	view, _ := b.o.hold(b)
	view.SetPicture(false)
	view.SetLoading(false)
	view.SetNote(note)
	if art != nil {
		view.SetLogo(art)
	}
	return true
}

func (b *standby) Seek(time.Duration) {}
func (b *standby) CanSeek() bool      { return false }

func (b *standby) Label() string {
	b.o.mu.Lock()
	defer b.o.mu.Unlock()
	if b.o.sess == nil {
		return ""
	}
	return b.o.sess.Label
}

func (b *standby) Play()     {}
func (b *standby) Pause()    {}
func (b *standby) Stop()     { b.o.end() }
func (b *standby) Next()     {}
func (b *standby) Previous() {}
