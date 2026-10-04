package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/cast"
	"github.com/ygelfand/LANovo/internal/lib/cast/playback"
)

// The player states a screen reports, from a captured session.
const (
	statePlaying = 1
	statePaused  = 2
	stateLoading = 3
	stateStopped = 0
)

// station is one lounge screen's player: the phone's playlist and the track playing from it.
type station struct {
	theme   string
	env     cast.Env
	resolve *Resolver
	ctx     context.Context

	mu      sync.Mutex
	sess    *Session
	cur     *track
	opening context.CancelFunc
	want    Position
	held    bool
	ids     []string
	index   int
	list    string
	params  string
	ctt     string
	sender  string
	phones  []string

	prev, next *Entry
	upcoming   []queued
	known      map[string]Track

	asked, lastHeard time.Time
	commanded        bool
	app              string

	skipsFor string
	skips    []Segment

	loud chan struct{}
}

func (s *station) markAsked(app string) {
	s.mu.Lock()
	s.app = app
	s.asked, s.commanded = time.Now(), false
	s.lastHeard = s.asked
	s.mu.Unlock()
}

func (s *station) sinceAsked() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.asked.IsZero() {
		return 0
	}
	return time.Since(s.asked).Round(time.Millisecond)
}

func (s *station) touch() {
	s.mu.Lock()
	s.lastHeard = time.Now()
	s.mu.Unlock()
}

func (s *station) quiet(d time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur == nil && s.opening == nil && time.Since(s.lastHeard) >= d
}

func newStation(ctx context.Context, theme string, env cast.Env) *station {
	r := NewResolver(env.HTTP, env.Video)
	r.Resample = env.Resample
	s := &station{theme: theme, env: env, resolve: r, ctx: ctx, loud: make(chan struct{}, 1)}
	if env.VolumeChanged != nil {
		go s.tellVolume()
		context.AfterFunc(ctx, env.VolumeChanged(s.volumeMoved))
	}
	return s
}

func (s *station) volumeMoved() {
	select {
	case s.loud <- struct{}{}:
	default:
	}
}

func (s *station) tellVolume() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.loud:
			s.volume()
		}
	}
}

func (s *station) bind(sess *Session) {
	s.mu.Lock()
	s.sess = sess
	s.mu.Unlock()
}

func (s *station) attend() {
	if s.env.Output == nil {
		return
	}
	s.mu.Lock()
	label, alone := s.sender, len(s.phones) == 0
	s.mu.Unlock()
	if alone {
		s.env.Output.Attend(nil)
		return
	}
	s.env.Output.Attend(&playback.Session{Label: label, Logo: s.logo(), Pictured: pictured(s.theme)})
}

func (s *station) logo() string {
	s.mu.Lock()
	app := s.app
	s.mu.Unlock()
	if themes[app] == s.theme {
		return cast.Lookup(app).Icon
	}
	var ids []string
	for id, theme := range themes {
		if theme == s.theme {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return ""
	}
	return cast.Lookup(slices.Min(ids)).Icon
}

func (s *station) refused() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cast.Lookup(s.app).Refused()
}

func (s *station) label() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sender
}

func (s *station) around() (prev, next bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ids) > 1 {
		return s.index > 0, s.index+1 < len(s.ids)
	}
	return s.prev != nil, s.next != nil
}

// coming is what the card lists as up next.
type queued struct {
	Track
	index int
}

const queueShown = 5

func (s *station) coming() []playback.Upcoming {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]playback.Upcoming, 0, len(s.upcoming))
	for _, q := range s.upcoming {
		play := func() { s.step(1) }
		if q.index >= 0 {
			play = func() { s.jump(q.index) }
		}
		out = append(out, playback.Upcoming{Title: q.Title, Artist: q.Author, Length: q.Duration, Art: "https://i.ytimg.com/vi/" + q.ID + "/mqdefault.jpg", Play: play})
	}
	return out
}

func (s *station) describe(ctx context.Context, ids []string, from, to int) []queued {
	found := make([]*Track, to-from)
	var wg sync.WaitGroup
	for k := range found {
		id := ids[from+k]
		s.mu.Lock()
		info, ok := s.known[id]
		s.mu.Unlock()
		if ok {
			found[k] = &info
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			info, err := s.resolve.Info(ctx, id)
			if err != nil {
				slog.Info("youtube queue", "theme", s.theme, "video", id, "err", err)
				return
			}
			found[k] = &info
		}()
	}
	wg.Wait()

	keep := map[string]bool{}
	for _, id := range ids {
		keep[id] = true
	}
	s.mu.Lock()
	if s.known == nil {
		s.known = map[string]Track{}
	}
	for id := range s.known {
		if !keep[id] {
			delete(s.known, id)
		}
	}
	var out []queued
	for k, f := range found {
		if f == nil {
			continue
		}
		s.known[ids[from+k]] = *f
		out = append(out, queued{Track: *f, index: from + k})
	}
	s.mu.Unlock()
	return out
}

func (s *station) jump(i int) {
	s.mu.Lock()
	if i < 0 || i >= len(s.ids) {
		s.mu.Unlock()
		return
	}
	pos := Position{Video: s.ids[i], List: s.list, Index: i}
	s.mu.Unlock()
	s.start(pos, 0)
}

// handle acts on one lounge message.
func (s *station) handle(m Message) {
	f := fields(m.Payload)
	switch m.Name {
	case "remoteConnected":
		s.mu.Lock()
		s.sender = f["name"]
		if id := f["id"]; id != "" && !slices.Contains(s.phones, id) {
			s.phones = append(s.phones, id)
		}
		s.mu.Unlock()
		s.attend()
		s.report(s.state())
	case "remoteDisconnected":
		s.mu.Lock()
		s.phones = slices.DeleteFunc(s.phones, func(id string) bool { return id == f["id"] })
		s.mu.Unlock()
		s.attend()
	case "loungeStatus":
		s.send(
			Out{Name: "onHasPreviousNextChanged", Fields: s.navigation()},
			Out{Name: "onAutoplayModeChanged", Fields: map[string]string{"autoplayMode": "UNSUPPORTED"}},
		)
	case "getNowPlaying", "onUserActivity":
		s.report(s.state())
	case "setPlaylist":
		s.mu.Lock()
		first := !s.asked.IsZero() && !s.commanded
		s.commanded = true
		s.mu.Unlock()
		if first {
			slog.Info("youtube first command", "theme", s.theme, "since asked", s.sinceAsked())
		}
		s.setPlaylist(f)
	case "updatePlaylist":
		s.mu.Lock()
		if ids := split(f["videoIds"]); len(ids) > 0 {
			s.ids = ids
		}
		if f["listId"] != "" {
			s.list = f["listId"]
		}
		s.mu.Unlock()
		s.send(Out{Name: "onHasPreviousNextChanged", Fields: s.navigation()})
	case "play":
		s.play()
	case "pause":
		s.pause()
	case "seekTo":
		if at, err := strconv.ParseFloat(f["newTime"], 64); err == nil {
			s.seek(time.Duration(at * float64(time.Second)))
		}
	case "next":
		s.step(1)
	case "previous":
		s.step(-1)
	case "stopVideo":
		s.stop()
	case "getVolume":
		s.volume()
	case "setVolume":
		if v, err := strconv.Atoi(f["volume"]); err == nil && s.env.SetVolume != nil {
			s.env.SetVolume(v)
			s.volume()
		}
	}
}

func (s *station) setPlaylist(f map[string]string) {
	id := f["videoId"]
	ids := split(f["videoIds"])
	index, _ := strconv.Atoi(f["currentIndex"])
	if len(ids) == 0 && id != "" {
		ids, index = []string{id}, 0
	}
	if id == "" && index >= 0 && index < len(ids) {
		id = ids[index]
	}
	at, _ := strconv.ParseFloat(f["currentTime"], 64)
	params := f["params"]
	if params == "" {
		params = f["playerParams"]
	}

	s.mu.Lock()
	s.ids, s.ctt = ids, f["ctt"]
	s.mu.Unlock()

	if id != "" {
		s.start(Position{Video: id, List: f["listId"], Index: index, Params: params}, time.Duration(at*float64(time.Second)))
	}
}

// start plays a video from at, abandoning whatever was playing or opening.
func (s *station) start(pos Position, at time.Duration) { s.open(pos, at, false) }

// open is start, beginning paused when held.
func (s *station) open(pos Position, at time.Duration, held bool) {
	id := pos.Video
	if err := s.refused(); err != nil {
		slog.Info("youtube refused", "theme", s.theme, "video", id, "err", err)
		if s.env.Output != nil {
			s.env.Output.Failed(err)
		}
		s.report(stateStopped)
		return
	}
	s.end(true)

	s.mu.Lock()
	same := pos.List != "" && pos.List == s.list
	s.index, s.list, s.params, s.want, s.held = pos.Index, pos.List, pos.Params, pos, held
	s.prev, s.next = nil, nil
	if same {
		s.upcoming = slices.DeleteFunc(slices.Clone(s.upcoming), func(q queued) bool { return q.index <= pos.Index })
	} else {
		s.upcoming = nil
	}
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.opening = cancel
	s.mu.Unlock()

	s.reportLoading(id, at)

	go func() {
		t, err := openTrack(ctx, s.resolve, s, id, at)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("youtube playback", "theme", s.theme, "video", id, "err", err)
				if s.env.Output != nil {
					s.env.Output.Failed(err)
				}
				s.report(stateStopped)
			}
			return
		}
		s.mu.Lock()
		if ctx.Err() != nil {
			s.mu.Unlock()
			t.close()
			return
		}
		s.cur = t
		held := s.held
		s.mu.Unlock()

		slog.Info("youtube playing", "theme", s.theme, "video", id, "title", t.info.Title, "at", at, "since asked", s.sinceAsked(), "paused", held)
		if s.env.Output != nil {
			s.env.Output.Play(t)
			if held {
				s.env.Output.Pause(t)
			}
		}
		switch {
		case held:
			s.report(statePaused)
		case !t.Pictured() || s.env.Output == nil:
			s.report(statePlaying)
		}
		s.lookAround(ctx, t, pos)
	}()
}

// lookAround asks what is either side of the track in the phone's queue, and fetches the next one's
// title for the card.
func (s *station) lookAround(ctx context.Context, t *track, pos Position) {
	s.mu.Lock()
	phones := slices.Clone(s.phones)
	ids, index := slices.Clone(s.ids), s.index
	s.mu.Unlock()

	var prev, next *Entry
	var up []queued
	switch {
	case len(ids) > 1:
		up = s.describe(ctx, ids, index+1, min(index+1+queueShown, len(ids)))
	case pos.List != "":
		var err error
		prev, next, err = neighbours(ctx, s.env.HTTP, pos, phones)
		if err != nil {
			slog.Info("youtube queue", "theme", s.theme, "list", pos.List, "index", pos.Index, "err", err)
			return
		}
		if next != nil {
			if info, err := s.resolve.Info(ctx, next.ID); err == nil {
				up = []queued{{Track: info, index: -1}}
			}
		}
	default:
		return
	}

	s.mu.Lock()
	if s.cur != t {
		s.mu.Unlock()
		return
	}
	s.prev, s.next, s.upcoming = prev, next, up
	s.mu.Unlock()

	s.changed(t)
	s.send(Out{Name: "onHasPreviousNextChanged", Fields: s.navigation()})
}

// halt stops what is playing or opening without telling the phone.
func (s *station) halt() { s.end(false) }

func (s *station) end(handoff bool) {
	s.mu.Lock()
	cur, opening := s.cur, s.opening
	s.cur, s.opening = nil, nil
	s.mu.Unlock()

	if opening != nil {
		opening()
	}
	if cur != nil {
		if s.env.Output != nil {
			if handoff {
				s.env.Output.Handoff(cur)
			} else {
				s.env.Output.Stop(cur)
			}
		}
		cur.close()
	}
}

func (s *station) play() {
	if s.whileOpening(false) {
		return
	}
	t := s.current()
	if t == nil {
		s.report(stateStopped)
		return
	}
	if s.env.Output != nil {
		s.env.Output.Resume(t)
	}
	s.report(statePlaying)
}

// whileOpening keeps a play or pause that arrives while the next track is still opening, for it to
// start in, and reports whether one was.
func (s *station) whileOpening(held bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cur != nil || s.opening == nil {
		return false
	}
	s.held = held
	return true
}

func (s *station) pause() {
	if s.whileOpening(true) {
		return
	}
	t := s.current()
	if t == nil {
		return
	}
	if s.env.Output != nil {
		s.env.Output.Pause(t)
	}
	s.report(statePaused)
}

func (s *station) paused(t *track) bool {
	if s.env.Output == nil {
		return false
	}
	_, paused := s.env.Output.Position(t)
	return paused
}

func (s *station) seek(at time.Duration) {
	s.mu.Lock()
	var pos Position
	cur, held := s.cur, s.held
	switch {
	case cur != nil:
		pos = Position{Video: cur.info.ID, List: s.list, Index: s.index, Params: s.params}
	case s.opening != nil:
		pos = s.want
	default:
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	if cur != nil {
		held = s.paused(cur)
	}
	s.open(pos, at, held)
}

// skipWhenHeard seeks past seg once playback has reached it.
func (s *station) skipWhenHeard(t *track, seg Segment) {
	until := time.Now().Add(skipWait)
	for time.Now().Before(until) {
		s.mu.Lock()
		cur := s.cur
		s.mu.Unlock()
		if cur != t || s.ctx.Err() != nil {
			return
		}
		if at, _ := s.env.Output.Position(t); at >= seg.From-skipEarly {
			break
		}
		time.Sleep(skipPoll)
	}
	slog.Info("youtube sponsorblock skip", "video", t.info.ID, "from", seg.From, "to", seg.To)
	s.seek(seg.To)
}

const (
	skipPoll  = 50 * time.Millisecond
	skipEarly = 100 * time.Millisecond
	skipWait  = 2 * time.Minute
)

// step moves through the phone's list when it sent one, or to what YouTube says is either side.
func (s *station) step(by int) {
	s.mu.Lock()
	var pos Position
	switch {
	case len(s.ids) > 1:
		i := s.index + by
		if i < 0 || i >= len(s.ids) {
			s.mu.Unlock()
			return
		}
		pos = Position{Video: s.ids[i], List: s.list, Index: i}
	case by > 0 && s.next != nil:
		pos = Position{Video: s.next.ID, List: s.next.List, Index: s.next.Index, Params: s.next.Params}
	case by < 0 && s.prev != nil:
		pos = Position{Video: s.prev.ID, List: s.prev.List, Index: s.prev.Index, Params: s.prev.Params}
	default:
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.start(pos, 0)
}

func (s *station) stop() {
	s.halt()
	s.report(stateStopped)
}

// finished is a track that has decoded to its end. What it queued plays out before the next starts.
func (s *station) finished(t *track) {
	for s.env.Output != nil && s.env.Output.Behind(t) > 0 {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
	if s.current() != t {
		return
	}
	_, next := s.around()
	if next {
		s.step(1)
		return
	}
	s.halt()
	s.report(stateStopped)
}

func (s *station) current() *track {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

func (s *station) changed(t *track) {
	if s.env.Output != nil {
		s.env.Output.Changed(t)
	}
}

func (s *station) state() int {
	t := s.current()
	if t == nil {
		return stateStopped
	}
	if s.paused(t) {
		return statePaused
	}
	return statePlaying
}

// heard is how far into the current track the room has actually heard.
func (s *station) heard(t *track) time.Duration {
	if s.env.Output == nil {
		return t.elapsed()
	}
	at, _ := s.env.Output.Position(t)
	return at
}

func (s *station) navigation() map[string]string {
	prev, next := s.around()
	return map[string]string{"hasPrevious": strconv.FormatBool(prev), "hasNext": strconv.FormatBool(next)}
}

func (s *station) reportLoading(id string, at time.Duration) {
	s.mu.Lock()
	list, index := s.list, s.index
	s.mu.Unlock()
	playing := map[string]string{
		"videoId": id, "currentTime": secs(at), "state": strconv.Itoa(stateLoading),
		"listId": list, "currentIndex": strconv.Itoa(index),
	}
	s.send(
		Out{Name: "nowPlaying", Fields: playing},
		Out{Name: "onStateChange", Fields: map[string]string{
			"state": strconv.Itoa(stateLoading), "currentTime": secs(at), "playabilityStatus": "OK",
		}},
	)
	s.publish(id, nil, stateLoading)
}

// report tells the phone what is playing and where it has got to.
func (s *station) report(state int) {
	t := s.current()
	if t == nil {
		s.send(Out{Name: "nowPlaying"}, Out{Name: "onStateChange", Fields: map[string]string{
			"state": strconv.Itoa(stateStopped), "playabilityStatus": "OK",
		}})
		s.publish("", nil, stateStopped)
		return
	}
	s.mu.Lock()
	list, index, ctt := s.list, s.index, s.ctt
	s.mu.Unlock()

	heard := s.heard(t)
	at, length, from := secs(heard), secs(t.info.Duration), "0"
	if oldest, edge, ok := t.window(); ok && !t.base.IsZero() {
		at, length, from = epoch(t.base.Add(heard)), epoch(edge), epoch(oldest)
	}
	st := strconv.Itoa(state)
	playing := map[string]string{
		"videoId": t.info.ID, "currentTime": at, "duration": length, "loadedTime": length,
		"seekableStartTime": from, "seekableEndTime": length, "state": st,
		"listId": list, "currentIndex": strconv.Itoa(index),
	}
	if ctt != "" {
		playing["ctt"] = ctt
	}
	s.send(
		Out{Name: "nowPlaying", Fields: playing},
		Out{Name: "onStateChange", Fields: map[string]string{
			"state": st, "currentTime": at, "duration": length, "loadedTime": length,
			"seekableStartTime": from, "seekableEndTime": length, "playabilityStatus": "OK",
		}},
		Out{Name: "onHasPreviousNextChanged", Fields: s.navigation()},
	)
	s.publish(t.info.ID, t, state)
}

func (s *station) volume() {
	v := 0
	if s.env.Volume != nil {
		v = s.env.Volume()
	}
	s.send(Out{Name: "onVolumeChanged", Fields: map[string]string{"volume": strconv.Itoa(v), "muted": "false"}})
}

func (s *station) discovery() {
	s.mu.Lock()
	sess := s.sess
	s.mu.Unlock()
	if sess == nil || s.env.Device.ID == "" {
		return
	}
	s.send(Out{Name: "setDiscoveryDeviceId", Fields: map[string]string{
		"discoveryDeviceId": strings.ToUpper(s.env.Device.ID),
		"loungeDeviceId":    sess.screen.Device,
		"castCloudDeviceId": s.env.Device.CloudID(),
	}})
}

func (s *station) send(out ...Out) {
	s.mu.Lock()
	sess := s.sess
	s.mu.Unlock()
	if sess == nil {
		return
	}
	if err := sess.Send(s.ctx, out...); err != nil {
		slog.Warn("youtube lounge send", "theme", s.theme, "err", err)
	}
}

// fields reads a lounge payload's values as strings, which is how the lounge sends them.
func fields(raw json.RawMessage) map[string]string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		switch v := v.(type) {
		case string:
			out[k] = v
		case nil:
		default:
			out[k] = fmt.Sprint(v)
		}
	}
	return out
}

func split(ids string) []string {
	if ids == "" {
		return nil
	}
	return strings.Split(ids, ",")
}

func secs(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
}

func epoch(t time.Time) string {
	return strconv.FormatFloat(float64(t.UnixMilli())/1000, 'f', 3, 64)
}
