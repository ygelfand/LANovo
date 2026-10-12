package speaker

import (
	"errors"
	"io"
	"testing"
	"testing/synctest"
)

func TestSilenceWaitsForPlaybackStartup(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		type result struct {
			stop func() error
			err  error
		}
		returned := make(chan result, 1)
		go func() {
			var started bool
			stop, err := silenceWriter(func(buf []byte) (int, error) {
				if started {
					return 0, io.EOF
				}
				started = true
				close(entered)
				<-release
				return len(buf), nil
			})
			returned <- result{stop: stop, err: err}
		}()
		<-entered
		synctest.Wait()
		var got result
		var hasReturned bool
		select {
		case got = <-returned:
			hasReturned = true
			t.Error("silenceWriter(blocked first write) returned before playback started")
		default:
		}
		close(release)
		if !hasReturned {
			got = <-returned
		}
		if got.err != nil {
			t.Fatalf("silenceWriter(successful first write) error = %v, want nil", got.err)
		}
		// Let the background writer finish; stop must join it and report its error.
		synctest.Wait()
		if err := got.stop(); !errors.Is(err, io.EOF) {
			t.Errorf("stop(after writer failure) = %v, want EOF", err)
		}
	})
}

func TestSilenceRejectsFailedPlaybackStartup(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		n    int
		err  error
		want error
	}{
		{name: "write failed", err: io.ErrClosedPipe, want: io.ErrClosedPipe},
		{name: "no frames written", want: io.ErrNoProgress},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			stop, err := silenceWriter(func([]byte) (int, error) {
				calls++
				return tt.n, tt.err
			})
			if stop != nil || !errors.Is(err, tt.want) {
				t.Errorf("silenceWriter(%s) = (stop present=%v, %v), want (false, %v)",
					tt.name, stop != nil, err, tt.want)
			}
			if calls != 1 {
				t.Errorf("silenceWriter(%s) wrote %d times, want 1", tt.name, calls)
			}
		})
	}
}
