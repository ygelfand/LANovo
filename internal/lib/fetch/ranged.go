package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Ranged reads a url front to back in fixed-size range requests, keeping at most a set number of
// chunks fetched ahead of the reader, so a long stream is never held whole.
type Ranged struct {
	client *http.Client
	url    string
	chunk  int64
	ahead  int
	parent context.Context
	ask    Ask
	renew  func(ctx context.Context) (string, error)
	rate   int64
	lead   time.Duration
	retry  time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	chunks chan piece
	cur    []byte
	size   int64
	closed bool
}

type piece struct {
	data []byte
	err  error
}

type Ask func(req *http.Request)

// Chunks is how a Ranged fetches.
type Chunks struct {
	// Size is each request's length, and Ahead how many may be fetched ahead of the reader.
	Size  int64
	Ahead int

	Ask Ask

	// Renew, if set, is asked for a fresh url when a chunk is refused with 403, and the chunk is
	// tried once more with it.
	Renew func(ctx context.Context) (string, error)

	Rate  int64
	Lead  time.Duration
	Retry time.Duration
}

const retries = 6

// NewRanged starts fetching url from the beginning. size is the length if known, else 0.
func NewRanged(ctx context.Context, client *http.Client, url string, size int64, c Chunks) *Ranged {
	if c.Ahead < 1 {
		c.Ahead = 1
	}
	r := &Ranged{
		client: client, url: url, chunk: c.Size, ahead: c.Ahead, parent: ctx, size: size,
		ask: c.Ask, renew: c.Renew, rate: c.Rate, lead: c.Lead, retry: c.Retry,
	}
	r.start(0)
	return r
}

// Size is the length, once known.
func (r *Ranged) Size() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}

func (r *Ranged) start(from int64) {
	ctx, cancel := context.WithCancel(r.parent)
	chunks := make(chan piece, r.ahead)
	r.cancel, r.chunks, r.cur = cancel, chunks, nil
	go r.fetch(ctx, chunks, from)
}

func (r *Ranged) fetch(ctx context.Context, out chan<- piece, from int64) {
	defer close(out)
	began := time.Now()
	for off := from; ; off += r.chunk {
		r.mu.Lock()
		size := r.size
		r.mu.Unlock()
		if size > 0 && off >= size {
			return
		}
		if !wait(ctx, time.Until(began.Add(r.due(off-from)))) {
			return
		}

		data, total, err := r.get(ctx, off, off > from)
		if total > 0 {
			r.mu.Lock()
			r.size = total
			r.mu.Unlock()
		}
		select {
		case out <- piece{data: data, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil || int64(len(data)) < r.chunk {
			return
		}
	}
}

func (r *Ranged) due(into int64) time.Duration {
	if r.rate <= 0 {
		return 0
	}
	return time.Duration(into)*time.Second/time.Duration(r.rate) - r.lead
}

func wait(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (r *Ranged) get(ctx context.Context, off int64, later bool) ([]byte, int64, error) {
	data, total, err := r.once(ctx, off)
	for n := 0; later && r.rate > 0 && errors.Is(err, errRefused) && n < retries; n++ {
		slog.Info("fetch retry", "at", off, "attempt", n+1, "in", r.retry)
		if !wait(ctx, r.retry) {
			return nil, 0, ctx.Err()
		}
		data, total, err = r.once(ctx, off)
	}
	if !errors.Is(err, errRefused) || r.renew == nil {
		return data, total, err
	}
	slog.Info("fetch renew", "at", off)
	fresh, rerr := r.renew(ctx)
	if rerr != nil {
		return nil, 0, fmt.Errorf("%w; renewing: %v", err, rerr)
	}
	r.mu.Lock()
	r.url = fresh
	r.mu.Unlock()
	return r.once(ctx, off)
}

// errRefused is a chunk the server answered with 403.
var errRefused = errors.New("fetch: refused")

func (r *Ranged) once(ctx context.Context, off int64) ([]byte, int64, error) {
	r.mu.Lock()
	url := r.url
	r.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	to := off + r.chunk - 1
	span := fmt.Sprintf("%d-%d", off, to)
	req.Header.Set("Range", "bytes="+span)
	if r.ask != nil {
		r.ask(req)
	}

	began := time.Now()
	resp, err := r.client.Do(req)
	if err != nil {
		slog.Warn("fetch", "range", span, "size", r.chunk, "err", err, "took", time.Since(began))
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		slog.Warn("fetch", "range", span, "size", r.chunk, "status", resp.Status, "took", time.Since(began),
			"url", url, "asked", req.Header, "answered", resp.Header, "body", string(body))
	}

	switch {
	case resp.StatusCode == http.StatusPartialContent:
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		return nil, 0, nil
	case resp.StatusCode == http.StatusOK && off == 0:
		data, err := io.ReadAll(io.LimitReader(resp.Body, r.chunk+1))
		slog.Info("fetch", "range", span, "size", r.chunk, "status", resp.Status, "got", len(data), "took", time.Since(began))
		if int64(len(data)) > r.chunk {
			return nil, 0, fmt.Errorf("fetch: %s ignores ranges", url)
		}
		return data, int64(len(data)), err
	case resp.StatusCode == http.StatusForbidden:
		return nil, 0, fmt.Errorf("%w: range %s: %s", errRefused, span, resp.Status)
	default:
		return nil, 0, fmt.Errorf("fetch: range %s: %s", span, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, r.chunk))
	slog.Info("fetch", "range", span, "size", r.chunk, "status", resp.Status, "got", len(data),
		"content-range", resp.Header.Get("Content-Range"), "took", time.Since(began), "err", err)
	return data, total(resp.Header.Get("Content-Range")), err
}

// total reads the length out of a Content-Range header, 0 when it does not say.
func total(h string) int64 {
	_, after, ok := strings.Cut(h, "/")
	if !ok || after == "*" {
		return 0
	}
	n, _ := strconv.ParseInt(after, 10, 64)
	return n
}

// Read hands over bytes in order, waiting for the next chunk when the current one is spent.
func (r *Ranged) Read(p []byte) (int, error) {
	for {
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return 0, io.ErrClosedPipe
		}
		if len(r.cur) > 0 {
			n := copy(p, r.cur)
			r.cur = r.cur[n:]
			r.mu.Unlock()
			return n, nil
		}
		chunks := r.chunks
		r.mu.Unlock()

		next, ok := <-chunks
		if !ok {
			return 0, io.EOF
		}
		if next.err != nil {
			return 0, next.err
		}
		if len(next.data) == 0 {
			return 0, io.EOF
		}

		r.mu.Lock()
		if r.chunks == chunks {
			r.cur = next.data
		}
		r.mu.Unlock()
	}
}

// From drops what is ahead and starts again from a byte offset.
func (r *Ranged) From(off int64) {
	slog.Info("fetch from", "at", off)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancel()
	r.start(off)
}

func (r *Ranged) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	r.cancel()
	return nil
}
