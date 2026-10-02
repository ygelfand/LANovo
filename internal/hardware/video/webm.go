package video

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/webm"
)

type webmFrames struct {
	r     *webm.Reader
	from  time.Duration
	keyed bool
}

func (w *webmFrames) Next() (Frame, error) {
	for {
		f, err := w.r.Next()
		if err != nil {
			return Frame{}, err
		}
		at := time.Duration(f.Time)
		if !w.keyed {
			if !f.Key || at+time.Second < w.from {
				continue
			}
			w.keyed = true
		}
		return Frame{Data: f.Data, At: at}, nil
	}
}

func WebM(r io.Reader, from time.Duration) (Stream, error) {
	demux := webm.NewVideoReader(r)
	tr, err := demux.Track()
	if err != nil {
		return Stream{}, err
	}
	var codec uint32
	switch tr.Codec {
	case "V_VP9":
		codec = VP9
	default:
		return Stream{}, fmt.Errorf("video: %s is not decoded here", tr.Codec)
	}
	frames := &webmFrames{r: demux, from: from}
	if s, ok := r.(interface{ From(int64) }); ok && from > 0 {
		if err := demux.Index(); err != nil {
			return Stream{}, err
		}
		off, at, ok := demux.Cue(int64(from))
		slog.Info("video cue", "want", from, "cue", time.Duration(at), "at", off, "found", ok)
		if ok {
			s.From(off)
			demux.Restart(r, off)
			frames.from = time.Duration(at)
		}
	}
	return Stream{
		Codec: codec, Width: uint32(tr.Width), Height: uint32(tr.Height),
		Source: frames,
	}, nil
}
