package logd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ygelfand/LANovo/internal/hardware/metrics"
)

const streamDepth = 256

type Handler struct {
	conn     *conn
	fallback io.Writer
	attrs    []slog.Attr
	group    string

	mu *sync.Mutex
}

type Line struct {
	Level slog.Level
	Text  string

	Dropped uint64
}

type stream struct {
	lines   chan Line
	dropped atomic.Uint64
}

var out = stream{lines: make(chan Line, streamDepth)}

func Lines() <-chan Line { return out.lines }

func (s *stream) publish(l Line) {
	l.Dropped = s.dropped.Swap(0)

	select {
	case s.lines <- l:
		return
	default:
	}

	select {
	case <-s.lines:
		s.dropped.Add(1)
	default:
	}

	select {
	case s.lines <- l:
	default:
		s.dropped.Add(1 + l.Dropped)
	}
}

func NewHandler(tag string, fallback io.Writer) *Handler {
	h := &Handler{fallback: fallback, mu: &sync.Mutex{}}

	c, err := dial(tag)
	if err != nil {
		fmt.Fprintf(fallback, "logcat unavailable: %v\n", err)
		return h
	}
	h.conn = c
	return h
}

func (h *Handler) Close() error {
	if h.conn == nil {
		return nil
	}
	return h.conn.Close()
}

func (h *Handler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= threshold.Level()
}

var threshold slog.LevelVar

func SetThreshold(l slog.Level) { threshold.Set(l) }

func Threshold() slog.Level { return threshold.Level() }

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := *h
	out.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &out
}

func (h *Handler) WithGroup(name string) slog.Handler {
	out := *h
	out.group = name
	return &out
}

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	fmt.Fprintf(&b, "[%8.2f] %s", metrics.Uptime(), r.Message)

	for _, a := range h.attrs {
		appendAttr(&b, h.group, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		appendAttr(&b, h.group, a)
		return true
	})

	line := b.String()

	out.publish(Line{Level: r.Level, Text: line})

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.fallback != nil {
		fmt.Fprintf(h.fallback, "%s %s\n", Letter(r.Level), line)
	}
	if h.conn == nil {
		return nil
	}
	return h.conn.write(Priority(r.Level), line)
}

func appendAttr(b *strings.Builder, group string, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}
	b.WriteByte(' ')
	if group != "" {
		b.WriteString(group)
		b.WriteByte('.')
	}
	b.WriteString(a.Key)
	b.WriteByte('=')
	b.WriteString(value(a.Value))
}

func value(v slog.Value) string {
	s := v.String()
	if strings.ContainsAny(s, " \t=\"") {
		return strconv.Quote(s)
	}
	return s
}
