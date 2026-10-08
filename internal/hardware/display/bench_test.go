package display

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestABenchReportsWhatOneFrameCost(t *testing.T) {
	b := Bench{Frames: 100, Draw: 600 * time.Millisecond, Pan: 150 * time.Millisecond}

	draw, pan := b.Each()
	if draw != 6*time.Millisecond {
		t.Errorf("a frame drew in %v, want 6ms", draw)
	}
	if pan != 1500*time.Microsecond {
		t.Errorf("a frame panned in %v, want 1.5ms", pan)
	}

	if rate := b.Rate(); rate < 133 || rate > 134 {
		t.Errorf("the rate is %.1f fps, want about 133.3", rate)
	}
}

func TestAnEmptyBench(t *testing.T) {
	var b Bench

	if draw, pan := b.Each(); draw != 0 || pan != 0 {
		t.Errorf("an empty bench costs %v and %v", draw, pan)
	}
	if b.Rate() != 0 {
		t.Errorf("an empty bench runs at %v fps", b.Rate())
	}
}

func TestABenchThatTookNoTime(t *testing.T) {
	b := Bench{Frames: 10}
	if b.Rate() != 0 {
		t.Errorf("ten frames in no time reported %v fps", b.Rate())
	}
}

func TestABenchWithNothingRenderingGivesUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := NewDriver("/dev/null").Bench(ctx, 10)
	if err == nil {
		t.Fatal("a measurement nothing could run reported success")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("gave up with %v, want the deadline", err)
	}
}

func TestABenchOfNoFrames(t *testing.T) {
	if _, err := NewDriver("/dev/null").Bench(context.Background(), 0); err == nil {
		t.Error("asking for no frames was accepted")
	}
}
