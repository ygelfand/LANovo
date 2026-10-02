package a2dp

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/lib/bt"
	"github.com/ygelfand/LANovo/internal/lib/bt/avrcp"
	"github.com/ygelfand/LANovo/internal/lib/bt/l2cap"
)

// What is lined up behind the current track, off the browsing channel.
//
// Two messages: ask how many entries the now playing scope holds, then ask for them. The count comes
// first because a phone refuses a range that runs past the end. The scope belongs to the addressed
// player, so the channel is not pointed at one.

// listing is how many entries are worth asking for.
const listing = 50

// labels keep the questions apart on the channel.
const (
	labelCount = iota
	labelFolder
	labelItem
)

// ahead is how many entries are asked about for their artwork. The answer is one round trip each
// and the card shows a handful, so the rest can wait until they are nearer.
const ahead = 6

// open asks for the browsing channel, if it is not up already.
func (s *Sink) open() {
	if s.browsable() {
		return
	}
	s.tell("opening the browsing channel", (*bt.Sink).OpenBrowsing)
}

// patience is how long a sequence has to finish before another may start.
const patience = 5 * time.Second

// list asks how long the list is, which is what the entries are then asked for by.
func (s *Sink) list() {
	if !s.browsable() {
		return
	}

	s.mu.Lock()
	busy := time.Since(s.asking) < patience
	if !busy {
		s.asking = time.Now()
	}
	s.mu.Unlock()

	if busy {
		return
	}

	s.tell("counting the now playing list", func(sink *bt.Sink) ([]l2cap.Frame, error) {
		return sink.Browse([]avrcp.Transport{
			avrcp.TotalItems(avrcp.ScopeNowPlaying).Request(labelCount),
		})
	})
}

// folder asks for the entries themselves. The range is inclusive at both ends.
func (s *Sink) folder(n uint32) {
	s.tell(fmt.Sprintf("asking for %d queued", n), func(sink *bt.Sink) ([]l2cap.Frame, error) {
		return sink.Browse([]avrcp.Transport{
			avrcp.FolderItems(avrcp.ScopeNowPlaying, 0, n-1).Request(labelFolder),
		})
	})
}

// listed takes one answer off the browsing channel and asks the next question.
func (s *Sink) listed(b avrcp.Browse) {
	switch b.ID {
	case avrcp.BrowseTotalItems:
		n, err := avrcp.ParseTotalItems(b.Params)
		if err != nil {
			slog.Warn("bluetooth browsing", "err", err)
			s.done()
			return
		}

		if n == 0 {
			s.holds(nil)
			s.done()
			return
		}
		s.folder(min(n, listing))

	case avrcp.BrowseItemAttrs:
		s.attributed(b)

	case avrcp.BrowseFolderItems:
		s.done()

		items, err := avrcp.ParseFolderItems(b.Params)
		if err != nil {
			slog.Warn("bluetooth browsing listing", "err", err, "params", brief(b.Params))
			return
		}

		// The listing carries the artwork only for what is playing, so the rest are asked about one
		// at a time.
		counter, _ := avrcp.Counter(b.Params)
		s.walk(counter, items)

		slog.Info("bluetooth queued", "items", len(items))
		if tracing() {
			for i, it := range items {
				slog.Info("bluetooth queued", "at", i, "uid", it.UID,
					"title", it.Track.Title, "artist", it.Track.Artist, "art", it.Track.Art)
			}
		}
		s.holds(items)

	default:
		status := byte(0)
		if len(b.Params) > 0 {
			status = b.Params[0]
		}
		slog.Info("bluetooth browsing answered", "pdu", fmt.Sprintf("%#02x", b.ID),
			"status", fmt.Sprintf("%#02x", status))
	}
}

// done ends the sequence, so the next track change starts another.
func (s *Sink) done() {
	s.mu.Lock()
	s.asking = time.Time{}
	s.mu.Unlock()
}

// holds keeps the list and tells the card there is something new to draw.
//
// The handles go with it. A uid is a position rather than a name — the same few come round as the
// window moves — so one kept from an older listing names whatever is at that position now.
func (s *Sink) holds(items []avrcp.Item) {
	s.mu.Lock()
	s.queue, s.handles = items, nil
	s.mu.Unlock()

	media.Get().Changed()
}

// browsable reports whether the browsing channel is up.
func (s *Sink) browsable() bool {
	s.mu.Lock()
	sink := s.sink
	s.mu.Unlock()

	return sink != nil && sink.Browsing()
}

// nothing is the uid a phone sends for a track change when it has stopped.
const nothing = ^uint64(0)

// queued is what follows the track that is playing, for the card.
func (s *Sink) queued(state avrcp.State) []media.Track {
	s.mu.Lock()
	items := s.queue
	s.mu.Unlock()

	s.mu.Lock()
	handles := s.handles
	s.mu.Unlock()

	out := make([]media.Track, 0, len(items))
	for _, it := range after(items, state) {
		out = append(out, media.Track{
			Title:  it.Track.Title,
			Artist: it.Track.Artist,
			Length: it.Track.Duration,
			Art:    s.image(handles[it.UID]),
		})
	}
	return out
}

// after is the entries behind the one playing. The phone says which that is by uid where it keeps
// a list of its own; where it does not, the title is what there is to match on. Matching nothing
// leaves the whole list, which is a list the phone has not started yet.
func after(items []avrcp.Item, state avrcp.State) []avrcp.Item {
	known := state.UID != 0 && state.UID != nothing

	for i, it := range items {
		switch {
		case known && it.UID == state.UID:
			return items[i+1:]
		case !known && state.Track.Title != "" && it.Track.Title == state.Track.Title:
			return items[i+1:]
		}
	}
	return items
}

// The artwork for what is lined up. A listing names the handle for the track that is playing and
// nothing for the rest, so each entry is asked about by uid, in order, one answer at a time.

// walk starts asking about the entries of a listing.
func (s *Sink) walk(counter uint16, items []avrcp.Item) {
	uids := make([]uint64, 0, ahead)
	for _, it := range items {
		if it.Kind != avrcp.ItemElement || len(uids) == ahead {
			continue
		}
		uids = append(uids, it.UID)
	}

	s.mu.Lock()
	running := s.at != 0 && time.Since(s.walked) < patience
	if running {
		s.later, s.waiting = uids, counter
	} else {
		s.counter, s.asks = counter, uids
		s.asked, s.got = len(uids), 0
	}
	s.mu.Unlock()

	s.expire()

	if !running {
		s.describe()
	}
}

// describe asks the next entry what it is.
func (s *Sink) describe() {
	s.mu.Lock()
	if len(s.asks) == 0 {
		s.at = 0
		got, asked, queued := s.got, s.asked, len(s.queue)

		// A listing that arrived while this one was running goes now.
		next, counter := s.later, s.waiting
		s.later = nil

		if len(next) > 0 {
			s.counter, s.asks = counter, next
			s.asked, s.got = len(next), 0
		}
		s.mu.Unlock()

		slog.Info("bluetooth entry artwork", "found", got, "asked", asked, "queued", queued)

		if len(next) > 0 {
			s.describe()
		}
		return
	}

	uid, counter := s.asks[0], s.counter
	s.at, s.asks, s.walked = uid, s.asks[1:], time.Now()
	s.mu.Unlock()

	// Every attribute rather than the artwork alone: a target that leaves one out of a list it was
	// asked for by name gives no sign of having done so.
	s.tell(fmt.Sprintf("asking about entry %d", uid), func(sink *bt.Sink) ([]l2cap.Frame, error) {
		return sink.Browse([]avrcp.Transport{
			avrcp.ItemAttributes(avrcp.ScopeNowPlaying, uid, counter).Request(labelItem),
		})
	})
}

// attributed takes what one entry said about itself and asks about the next.
func (s *Sink) attributed(b avrcp.Browse) {
	s.mu.Lock()
	uid := s.at
	s.mu.Unlock()

	track, err := avrcp.ParseItemAttributes(b.Params)
	if err != nil {
		slog.Warn("bluetooth browsing entry", "uid", uid, "err", err,
			"answer", fmt.Sprintf("% x", b.Params))
		s.describe()
		return
	}

	if tracing() {
		slog.Info("bluetooth entry", "uid", uid, "title", track.Title, "artist", track.Artist,
			"album", track.Album, "art", track.Art, "answer", fmt.Sprintf("% x", b.Params))
	}

	if track.Art != "" {
		s.mu.Lock()
		if s.handles == nil {
			s.handles = map[uint64]string{}
		}
		s.handles[uid] = track.Art
		s.got++
		s.mu.Unlock()

		s.wants(track.Art, false)
	}

	s.describe()
}
