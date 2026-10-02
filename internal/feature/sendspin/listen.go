package sendspin

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

const Port = 8928

// listener accepts the servers that dial in, one at a time. The spec ranks competing servers by
// declared activity; until that is implemented the first to arrive holds the room.
type listener struct {
	out *out
	bg  *speaker.Arbiter

	// player is what the room shows Home Assistant. Written to from the accept goroutine and from the
	// session, so it has to tolerate that.
	player *Player

	mu   sync.Mutex
	busy bool
}

func newListener(o *out, bg *speaker.Arbiter, p *Player) *listener {
	return &listener{out: o, bg: bg, player: p}
}

// serve holds the port until ctx ends.
func (l *listener) serve(ctx context.Context, name string) error {
	ln, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(Port)))
	if err != nil {
		return err
	}

	up := websocket.Upgrader{
		// Any origin: a server dialing in is not a browser, and there is nothing here a page could
		// reach that the network cannot already.
		CheckOrigin: func(*http.Request) bool { return true },
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if !l.take() {
			http.Error(w, "already connected", http.StatusConflict)
			return
		}
		defer l.give()

		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			slog.Warn("sendspin upgrade failed", "from", r.RemoteAddr, "err", err)
			return
		}
		defer conn.Close()

		slog.Info("sendspin server connected", "from", r.RemoteAddr)
		l.player.state.Set(stateJoined)
		defer l.player.state.Set(stateWaiting)

		// For as long as the server is connected, not just while audio is arriving: a paused group is
		// one this room can still be told to play again.
		s := newSession(conn, l.out, l.bg, name, l.player)
		l.player.holds(s)
		defer l.player.holds(nil)

		if err := s.run(ctx); err != nil {
			slog.Warn("sendspin session ended", "from", r.RemoteAddr, "err", err)
		} else {
			slog.Info("sendspin server disconnected", "from", r.RemoteAddr)
		}
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	// Closing the server unblocks Serve and hangs up on whatever is connected, which is what stopping
	// means here: the room is leaving the group, not pausing.
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// take admits one server and turns away the rest.
func (l *listener) take() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.busy {
		return false
	}
	l.busy = true
	return true
}

func (l *listener) give() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.busy = false
}
